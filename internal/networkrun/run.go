package networkrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/audit"
	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/protocolclient"
	"github.com/khicago/supermover/internal/transport"
)

type Options struct {
	TargetRoot           string
	ArtifactWriter       ArtifactWriter
	ProfileSnapshot      *control.ProfileSnapshot
	ProfilePrivacyPolicy profile.PrivacyPolicy
	Request              protocolclient.TransferRequest
	Client               protocolclient.Client
	Now                  func() time.Time
}

type Result struct {
	ClientResult protocolclient.Result
	Transfer     control.NetworkTransfer
}

var writeControlFile = control.WriteFile

var (
	ErrPayloadOverheadMissing = errors.New("payload privacy overhead evidence is missing")
)

type ArtifactWriter interface {
	CanWriteBeforeBegin() bool
	WriteProfileSnapshot(context.Context, control.ProfileSnapshot) error
	WriteWarnings(context.Context, []control.Warning) error
	WriteNetworkTransfer(context.Context, control.NetworkTransfer) error
}

type NetworkTransferReader interface {
	ReadNetworkTransfer(context.Context, string) (control.NetworkTransfer, error)
}

type LocalArtifactWriter struct {
	TargetRoot string
}

func (w LocalArtifactWriter) CanWriteBeforeBegin() bool {
	return true
}

func (w LocalArtifactWriter) WriteProfileSnapshot(_ context.Context, doc control.ProfileSnapshot) error {
	path, err := control.Path(w.TargetRoot, control.ArtifactProfileSnapshot, doc.ID)
	if err != nil {
		return err
	}
	return writeControlFile(path, doc)
}

func (w LocalArtifactWriter) WriteWarnings(_ context.Context, docs []control.Warning) error {
	for _, doc := range docs {
		path, err := control.Path(w.TargetRoot, control.ArtifactWarning, doc.ID)
		if err != nil {
			return err
		}
		if err := writeControlFile(path, doc); err != nil {
			return err
		}
	}
	return nil
}

func (w LocalArtifactWriter) WriteNetworkTransfer(_ context.Context, doc control.NetworkTransfer) error {
	path, err := control.Path(w.TargetRoot, control.ArtifactNetworkTransfer, doc.SessionID)
	if err != nil {
		return err
	}
	return writeControlFile(path, doc)
}

func (w LocalArtifactWriter) ReadNetworkTransfer(_ context.Context, sessionID string) (control.NetworkTransfer, error) {
	path, err := control.Path(w.TargetRoot, control.ArtifactNetworkTransfer, sessionID)
	if err != nil {
		return control.NetworkTransfer{}, err
	}
	doc, err := control.ReadFileNoSymlinkUnderRoot[control.NetworkTransfer](w.TargetRoot, path)
	if err != nil {
		return control.NetworkTransfer{}, err
	}
	if doc.SessionID != sessionID {
		return control.NetworkTransfer{}, fmt.Errorf("network transfer artifact session_id = %q, want %q", doc.SessionID, sessionID)
	}
	return doc, nil
}

type HTTPArtifactWriter struct {
	BaseURL string
	Doer    protocolclient.Doer
}

func artifactWriter(opts Options) (ArtifactWriter, error) {
	if opts.ArtifactWriter != nil {
		return opts.ArtifactWriter, nil
	}
	if strings.TrimSpace(opts.TargetRoot) == "" {
		return nil, errors.New("target root is required")
	}
	return LocalArtifactWriter{TargetRoot: opts.TargetRoot}, nil
}

func (w HTTPArtifactWriter) CanWriteBeforeBegin() bool {
	return false
}

func (w HTTPArtifactWriter) WriteProfileSnapshot(ctx context.Context, doc control.ProfileSnapshot) error {
	payload, err := marshalControlDocument(doc)
	if err != nil {
		return err
	}
	return w.postArtifact(ctx, doc.SessionID, "/artifacts/profile", protocol.ProfileSnapshotArtifactRequest{
		SessionID: doc.SessionID,
		Document:  payload,
	})
}

func (w HTTPArtifactWriter) WriteWarnings(ctx context.Context, docs []control.Warning) error {
	if len(docs) == 0 {
		return nil
	}
	sessionID := docs[0].SessionID
	payloads := make([][]byte, 0, len(docs))
	for _, doc := range docs {
		if doc.SessionID != sessionID {
			return fmt.Errorf("warning session_id %q does not match %q", doc.SessionID, sessionID)
		}
		payload, err := marshalControlDocument(doc)
		if err != nil {
			return err
		}
		payloads = append(payloads, payload)
	}
	return w.postArtifact(ctx, sessionID, "/artifacts/warnings", protocol.WarningArtifactRequest{
		SessionID: sessionID,
		Documents: payloads,
	})
}

func (w HTTPArtifactWriter) WriteNetworkTransfer(ctx context.Context, doc control.NetworkTransfer) error {
	payload, err := marshalControlDocument(doc)
	if err != nil {
		return err
	}
	return w.postArtifact(ctx, doc.SessionID, "/artifacts/network-transfer", protocol.NetworkTransferArtifactRequest{
		SessionID: doc.SessionID,
		Document:  payload,
	})
}

func (w HTTPArtifactWriter) ReadNetworkTransfer(ctx context.Context, sessionID string) (control.NetworkTransfer, error) {
	endpoint, err := artifactEndpoint(w.BaseURL)
	if err != nil {
		return control.NetworkTransfer{}, err
	}
	if w.Doer == nil {
		return control.NetworkTransfer{}, errors.New("artifact writer HTTP client is required")
	}
	reqPath := "/v1/sessions/" + url.PathEscape(sessionID) + "/artifacts/network-transfer"
	reqURL := endpoint.ResolveReference(&url.URL{Path: endpoint.Path + reqPath})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return control.NetworkTransfer{}, fmt.Errorf("build GET %s: %w", reqPath, err)
	}
	resp, err := w.Doer.Do(req)
	if err != nil {
		return control.NetworkTransfer{}, fmt.Errorf("GET %s: %w", reqPath, err)
	}
	if resp == nil {
		return control.NetworkTransfer{}, fmt.Errorf("GET %s: receiver returned nil response", reqPath)
	}
	if resp.Body == nil {
		resp.Body = io.NopCloser(strings.NewReader(""))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return control.NetworkTransfer{}, decodeRemoteArtifactError(http.MethodGet, reqPath, resp)
	}
	data, err := readBoundedArtifactResponse(resp.Body)
	if err != nil {
		return control.NetworkTransfer{}, fmt.Errorf("decode GET %s response: %w", reqPath, err)
	}
	doc, err := control.Read[control.NetworkTransfer](bytes.NewReader(data))
	if err != nil {
		return control.NetworkTransfer{}, fmt.Errorf("decode GET %s response: %w", reqPath, err)
	}
	if doc.SessionID != sessionID {
		return control.NetworkTransfer{}, fmt.Errorf("network transfer response session_id = %q, want %q", doc.SessionID, sessionID)
	}
	return doc, nil
}

func readBoundedArtifactResponse(reader io.Reader) ([]byte, error) {
	limited := &io.LimitedReader{R: reader, N: protocol.MaxArtifactDocumentBytes + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > protocol.MaxArtifactDocumentBytes {
		return nil, fmt.Errorf("network transfer artifact response exceeds maximum %d bytes", protocol.MaxArtifactDocumentBytes)
	}
	return data, nil
}

func (w HTTPArtifactWriter) postArtifact(ctx context.Context, sessionID string, suffix string, body any) error {
	endpoint, err := artifactEndpoint(w.BaseURL)
	if err != nil {
		return err
	}
	if w.Doer == nil {
		return errors.New("artifact writer HTTP client is required")
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("encode artifact request: %w", err)
	}
	reqPath := "/v1/sessions/" + url.PathEscape(sessionID) + suffix
	reqURL := endpoint.ResolveReference(&url.URL{Path: endpoint.Path + reqPath})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), &buf)
	if err != nil {
		return fmt.Errorf("build POST %s: %w", reqPath, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.Doer.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", reqPath, err)
	}
	if resp == nil {
		return fmt.Errorf("POST %s: receiver returned nil response", reqPath)
	}
	if resp.Body == nil {
		resp.Body = io.NopCloser(strings.NewReader(""))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decodeRemoteArtifactError(http.MethodPost, reqPath, resp)
	}
	var ack protocol.ArtifactWriteResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, protocol.MaxArtifactDocumentBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ack); err != nil {
		return fmt.Errorf("decode POST %s response: %w", reqPath, err)
	}
	if ack.SessionID != sessionID {
		return fmt.Errorf("artifact response session_id = %q, want %q", ack.SessionID, sessionID)
	}
	return nil
}

func artifactEndpoint(base string) (*url.URL, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return nil, errors.New("artifact writer base URL is required")
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse artifact writer base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("artifact writer base URL must include scheme and host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed, nil
}

func decodeRemoteArtifactError(method, path string, resp *http.Response) error {
	var remote protocol.ErrorResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&remote); err != nil {
		remote.Message = strings.TrimSpace(resp.Status)
	}
	return &protocolclient.RemoteError{
		Method:     method,
		Path:       path,
		StatusCode: resp.StatusCode,
		Code:       remote.Code,
		Message:    remote.Message,
	}
}

func marshalControlDocument(doc control.Document) ([]byte, error) {
	if err := doc.Validate(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := control.Write(&buf, doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Run(ctx context.Context, opts Options) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("context is required")
	}
	writer, err := artifactWriter(opts)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(opts.Request.SessionID) == "" {
		return Result{}, errors.New("session id is required")
	}
	profilePrivacyPolicy, err := transportPrivacyPolicyFromProfile(opts.ProfilePrivacyPolicy)
	if err != nil {
		return Result{}, err
	}
	if opts.Request.PrivacyPolicy.Level == 0 {
		return Result{}, errors.New("privacy policy is required")
	}
	if err := opts.Request.PrivacyPolicy.Validate(); err != nil {
		return Result{}, fmt.Errorf("privacy policy: %w", err)
	}
	if opts.Request.PrivacyPolicy != profilePrivacyPolicy {
		return Result{}, errors.New("privacy policy does not match profile privacy policy")
	}
	now := nowFunc(opts.Now)
	startedAt := now()
	transfer := newTransfer(opts.Request, startedAt)
	priorBeforeRun := priorNetworkTransferState{}
	if writer.CanWriteBeforeBegin() {
		prior, err := priorNetworkTransfer(ctx, writer, transfer.SessionID)
		if err != nil {
			return Result{}, err
		}
		priorBeforeRun = prior
		if opts.ProfileSnapshot != nil {
			if err := writer.WriteProfileSnapshot(ctx, *opts.ProfileSnapshot); err != nil {
				return Result{}, err
			}
		}
		if !priorBeforeRun.exists {
			if err := writer.WriteNetworkTransfer(ctx, transfer); err != nil {
				return Result{}, err
			}
		}
	}
	request := opts.Request
	progressState := &networkProgressState{
		prior:             priorBeforeRun,
		allowInFlight:     !priorBeforeRun.exists,
		writeStatusEvents: writer.CanWriteBeforeBegin(),
	}
	preflightResume := resumeEvidencePreflight(writer, transfer, progressState, now)
	request.Progress = chainProgress(chainProgress(preflightResume, networkTransferProgressWriter(writer, transfer, now, progressState)), request.Progress)
	clientResult, err := opts.Client.Run(ctx, request)
	if err != nil {
		if errors.Is(err, ErrPayloadOverheadMissing) && clientResult.Bytes == 0 && clientResult.Chunks == 0 {
			transfer = finishTransfer(transfer, outcome{
				status: control.NetworkTransferNeedsRepair,
				stage:  "network_transfer_artifact",
				code:   "payload_overhead_missing",
			}, now(), err)
			transfer.PrivacyOverhead = nil
			return Result{ClientResult: clientResult, Transfer: transfer}, err
		}
		failure := classifyError(err)
		if shouldMergeFailedRetry(clientResult, failure) {
			prior, priorErr := priorNetworkTransfer(ctx, writer, transfer.SessionID)
			if priorErr != nil {
				return writePayloadEvidenceFailure(ctx, writer, clientResult, transfer, now, errors.Join(err, priorErr))
			}
			if !prior.exists {
				return writePayloadEvidenceFailure(ctx, writer, clientResult, transfer, now, errors.Join(err, fmt.Errorf("%w: prior network transfer evidence is missing", ErrPayloadOverheadMissing)))
			}
			currentOverhead := privacyOverheadFromClient(clientResult.Privacy)
			if currentOverhead == nil {
				currentOverhead = privacyOverheadFromError(err)
			}
			if !hasCurrentPayloadOverhead(currentOverhead) && !commitOnlyPayloadRetry(clientResult) {
				mergeErr := fmt.Errorf("%w: retried transfer failed without current payload overhead counters", ErrPayloadOverheadMissing)
				return writePayloadEvidenceFailure(ctx, writer, clientResult, transfer, now, errors.Join(err, mergeErr))
			}
			transfer.PrivacyOverhead = currentOverhead
			transfer = finishTransfer(transfer, failure, now(), err)
			merged, mergeErr := mergeRetryAttempt(ctx, writer, transfer, currentOverhead, false)
			if mergeErr != nil {
				return writePayloadEvidenceFailure(ctx, writer, clientResult, transfer, now, errors.Join(err, mergeErr))
			}
			if writeErr := writer.WriteNetworkTransfer(ctx, merged); writeErr != nil {
				return Result{ClientResult: clientResult, Transfer: merged}, errors.Join(err, fmt.Errorf("write network transfer evidence: %w", writeErr))
			}
			return Result{ClientResult: clientResult, Transfer: merged}, err
		}
		transfer.PrivacyOverhead = privacyOverheadFromClient(clientResult.Privacy)
		if transfer.PrivacyOverhead == nil {
			transfer.PrivacyOverhead = privacyOverheadFromError(err)
		}
		transfer = finishTransfer(transfer, failure, now(), err)
		if writeErr := writer.WriteNetworkTransfer(ctx, transfer); writeErr != nil {
			return Result{ClientResult: clientResult, Transfer: transfer}, errors.Join(err, fmt.Errorf("write network transfer evidence: %w", writeErr))
		}
		return Result{ClientResult: clientResult, Transfer: transfer}, err
	}
	currentOverhead := privacyOverheadFromClient(clientResult.Privacy)
	transfer.PrivacyOverhead = currentOverhead
	prior, priorErr := priorNetworkTransfer(ctx, writer, transfer.SessionID)
	requiresPrior := priorErr != nil || requiresPriorPayloadEvidence(clientResult, currentOverhead)
	mergeResume := requiresPrior && prior.exists
	if !writer.CanWriteBeforeBegin() {
		if opts.ProfileSnapshot != nil {
			if err := writer.WriteProfileSnapshot(ctx, *opts.ProfileSnapshot); err != nil {
				return Result{ClientResult: clientResult, Transfer: transfer}, err
			}
		}
		if !payloadUploadSkipped(clientResult, currentOverhead) && !mergeResume {
			if err := writer.WriteNetworkTransfer(ctx, transfer); err != nil {
				return Result{ClientResult: clientResult, Transfer: transfer}, err
			}
		}
	}
	warnings := warningArtifacts(clientResult.SessionID, now(), clientResult.Warnings)
	if err := writer.WriteWarnings(ctx, warnings); err != nil {
		transfer = finishTransfer(transfer, outcome{
			status: control.NetworkTransferPublishFailed,
			stage:  "warning_artifacts",
			code:   "warning_artifact_write_failed",
		}, now(), err)
		if writeErr := writer.WriteNetworkTransfer(ctx, transfer); writeErr != nil {
			return Result{ClientResult: clientResult, Transfer: transfer}, errors.Join(err, fmt.Errorf("write network transfer evidence: %w", writeErr))
		}
		return Result{ClientResult: clientResult, Transfer: transfer}, err
	}
	if payloadUploadSkipped(clientResult, transfer.PrivacyOverhead) {
		if priorErr != nil {
			return writePayloadEvidenceFailure(ctx, writer, clientResult, transfer, now, priorErr)
		}
		if !prior.exists {
			return writePayloadEvidenceFailure(ctx, writer, clientResult, transfer, now, fmt.Errorf("%w: prior network transfer evidence is missing", ErrPayloadOverheadMissing))
		}
		merged, mergeErr := mergeSuccessfulRetry(ctx, writer, transfer, currentOverhead, now(), true)
		if mergeErr != nil {
			return writePayloadEvidenceFailure(ctx, writer, clientResult, transfer, now, mergeErr)
		}
		if err := writer.WriteNetworkTransfer(ctx, merged); err != nil {
			transfer = finishTransfer(transfer, outcome{
				status: control.NetworkTransferPublishFailed,
				stage:  "network_transfer_artifact",
				code:   "network_transfer_artifact_write_failed",
			}, now(), err)
			return Result{ClientResult: clientResult, Transfer: transfer}, err
		}
		return Result{ClientResult: clientResult, Transfer: merged}, nil
	}
	if requiresPrior {
		if priorErr != nil {
			return writePayloadEvidenceFailure(ctx, writer, clientResult, transfer, now, priorErr)
		}
		if !prior.exists {
			return writePayloadEvidenceFailure(ctx, writer, clientResult, transfer, now, fmt.Errorf("%w: prior network transfer evidence is missing", ErrPayloadOverheadMissing))
		}
		if !hasCurrentPayloadOverhead(currentOverhead) && !commitOnlyPayloadRetry(clientResult) {
			mergeErr := fmt.Errorf("%w: resumed transfer has no current payload overhead counters", ErrPayloadOverheadMissing)
			return writePayloadEvidenceFailure(ctx, writer, clientResult, transfer, now, mergeErr)
		}
		merged, mergeErr := mergeSuccessfulRetry(ctx, writer, transfer, currentOverhead, now(), false)
		if mergeErr != nil {
			return writePayloadEvidenceFailure(ctx, writer, clientResult, transfer, now, mergeErr)
		}
		if err := writer.WriteNetworkTransfer(ctx, merged); err != nil {
			transfer = finishTransfer(transfer, outcome{
				status: control.NetworkTransferPublishFailed,
				stage:  "network_transfer_artifact",
				code:   "network_transfer_artifact_write_failed",
			}, now(), err)
			return Result{ClientResult: clientResult, Transfer: transfer}, err
		}
		return Result{ClientResult: clientResult, Transfer: merged}, nil
	}
	pending := finishTransfer(transfer, outcome{
		status: control.NetworkTransferPublishFailed,
		stage:  "network_transfer_artifact",
		code:   "network_transfer_artifact_pending",
	}, now(), nil)
	if err := writer.WriteNetworkTransfer(ctx, pending); err != nil {
		transfer = finishTransfer(transfer, outcome{
			status: control.NetworkTransferPublishFailed,
			stage:  "network_transfer_artifact",
			code:   "network_transfer_artifact_write_failed",
		}, now(), err)
		return Result{ClientResult: clientResult, Transfer: transfer}, err
	}
	transfer = finishTransfer(pending, outcome{
		status: control.NetworkTransferPublished,
		stage:  "commit",
	}, now(), nil)
	if err := writer.WriteNetworkTransfer(ctx, transfer); err != nil {
		transfer = finishTransfer(transfer, outcome{
			status: control.NetworkTransferPublishFailed,
			stage:  "network_transfer_artifact",
			code:   "network_transfer_artifact_write_failed",
		}, now(), err)
		return Result{ClientResult: clientResult, Transfer: transfer}, err
	}
	return Result{ClientResult: clientResult, Transfer: transfer}, nil
}

func writePayloadEvidenceFailure(ctx context.Context, writer ArtifactWriter, clientResult protocolclient.Result, transfer control.NetworkTransfer, now func() time.Time, cause error) (Result, error) {
	transfer.PrivacyOverhead = nil
	transfer = finishTransfer(transfer, outcome{
		status: control.NetworkTransferNeedsRepair,
		stage:  "network_transfer_artifact",
		code:   "payload_overhead_missing",
	}, now(), cause)
	if writeErr := writer.WriteNetworkTransfer(ctx, transfer); writeErr != nil {
		return Result{ClientResult: clientResult, Transfer: transfer}, errors.Join(cause, fmt.Errorf("write network transfer evidence: %w", writeErr))
	}
	return Result{ClientResult: clientResult, Transfer: transfer}, cause
}

func transportPrivacyPolicyFromProfile(policy profile.PrivacyPolicy) (transport.PrivacyPolicy, error) {
	if policy.TrafficLevel == 0 {
		return transport.PrivacyPolicy{}, errors.New("profile privacy policy is required")
	}
	transportPolicy := transport.PrivacyPolicy{
		Level:            transport.PrivacyLevel(policy.TrafficLevel),
		PaddingBucket:    policy.PaddingBucketBytes,
		BatchMaxBytes:    policy.BatchMaxBytes,
		BatchMaxCount:    policy.BatchMaxCount,
		JitterBudget:     policy.JitterBudgetMillis,
		DiscoveryLowInfo: policy.DiscoveryLowInfo,
	}
	if err := transportPolicy.Validate(); err != nil {
		return transport.PrivacyPolicy{}, fmt.Errorf("profile privacy policy: %w", err)
	}
	return transportPolicy, nil
}

func privacyOverheadFromClient(in protocolclient.PrivacyOverhead) *control.NetworkTransferPrivacyOverhead {
	overhead := control.NetworkTransferPrivacyOverhead{
		FramePlainBytes:      in.FramePlainBytes,
		FrameWireBytes:       in.FrameWireBytes,
		PaddingBytes:         in.PaddingBytes,
		PaddedChunks:         in.PaddedChunks,
		PaddingBucketBytes:   in.PaddingBucketBytes,
		BatchFrames:          in.BatchFrames,
		BatchedChunks:        in.BatchedChunks,
		MaxBatchCount:        in.MaxBatchCount,
		MaxBatchPlainBytes:   in.MaxBatchPlainBytes,
		JitteredRequests:     in.JitteredRequests,
		JitterDelayMillis:    in.JitterDelayMillis,
		MaxJitterDelayMillis: in.MaxJitterDelayMillis,
		JitterBudgetMillis:   in.JitterBudgetMillis,
	}
	if overhead.Empty() {
		return nil
	}
	return &overhead
}

func privacyOverheadFromError(err error) *control.NetworkTransferPrivacyOverhead {
	var transferErr *protocolclient.TransferError
	if !errors.As(err, &transferErr) {
		return nil
	}
	return privacyOverheadFromClient(transferErr.Privacy)
}

func chainProgress(first, second protocolclient.ProgressCallback) protocolclient.ProgressCallback {
	if first == nil {
		return second
	}
	if second == nil {
		return first
	}
	return func(ctx context.Context, event protocolclient.ProgressEvent) error {
		if err := first(ctx, event); err != nil {
			return err
		}
		return second(ctx, event)
	}
}

type networkProgressState struct {
	prior             priorNetworkTransferState
	allowInFlight     bool
	writeStatusEvents bool
}

func resumeEvidencePreflight(writer ArtifactWriter, transfer control.NetworkTransfer, state *networkProgressState, now func() time.Time) protocolclient.ProgressCallback {
	var checked bool
	return func(ctx context.Context, event protocolclient.ProgressEvent) error {
		if event.Stage != protocolclient.ProgressStageStatus || !progressEventNeedsPriorPayloadEvidence(event) {
			return nil
		}
		if !checked && !writer.CanWriteBeforeBegin() {
			refreshed, err := priorNetworkTransfer(ctx, writer, transfer.SessionID)
			if err != nil {
				return writePreflightPayloadEvidenceFailure(ctx, writer, transfer, now, err)
			}
			state.prior = refreshed
			state.allowInFlight = !refreshed.exists
			checked = true
		}
		if !state.prior.exists {
			return writePreflightPayloadEvidenceFailure(ctx, writer, transfer, now, fmt.Errorf("%w: prior network transfer evidence is missing", ErrPayloadOverheadMissing))
		}
		if state.prior.doc.PrivacyOverhead == nil || !hasPayloadOverhead(*state.prior.doc.PrivacyOverhead) {
			return writePreflightPayloadEvidenceFailure(ctx, writer, transfer, now, fmt.Errorf("%w: prior network transfer has no payload overhead counters", ErrPayloadOverheadMissing))
		}
		if state.prior.doc.SessionID != transfer.SessionID {
			return writePreflightPayloadEvidenceFailure(ctx, writer, transfer, now, fmt.Errorf("%w: prior network transfer session_id %q does not match retry session_id %q", ErrPayloadOverheadMissing, state.prior.doc.SessionID, transfer.SessionID))
		}
		if state.prior.doc.ProfileID != transfer.ProfileID ||
			state.prior.doc.TargetID != transfer.TargetID ||
			state.prior.doc.SourceDeviceID != transfer.SourceDeviceID ||
			state.prior.doc.TargetDeviceID != transfer.TargetDeviceID ||
			state.prior.doc.ProtocolVersion != transfer.ProtocolVersion ||
			state.prior.doc.PrivacyPolicy != transfer.PrivacyPolicy {
			return writePreflightPayloadEvidenceFailure(ctx, writer, transfer, now, fmt.Errorf("%w: prior network transfer scope does not match retry request", ErrPayloadOverheadMissing))
		}
		return nil
	}
}

func progressEventNeedsPriorPayloadEvidence(event protocolclient.ProgressEvent) bool {
	if event.State == protocol.SessionStatePublished {
		return true
	}
	for _, file := range event.ResumeFrom {
		if file.CommittedSize > 0 || file.Complete {
			return true
		}
	}
	return false
}

func writePreflightPayloadEvidenceFailure(ctx context.Context, writer ArtifactWriter, transfer control.NetworkTransfer, now func() time.Time, cause error) error {
	failed := finishTransfer(transfer, outcome{
		status: control.NetworkTransferNeedsRepair,
		stage:  "network_transfer_artifact",
		code:   "payload_overhead_missing",
	}, now(), cause)
	failed.PrivacyOverhead = nil
	if err := writer.WriteNetworkTransfer(ctx, failed); err != nil {
		return errors.Join(cause, fmt.Errorf("write network transfer evidence: %w", err))
	}
	return cause
}

func networkTransferProgressWriter(writer ArtifactWriter, base control.NetworkTransfer, now func() time.Time, state *networkProgressState) protocolclient.ProgressCallback {
	return func(ctx context.Context, event protocolclient.ProgressEvent) error {
		if state == nil || !state.allowInFlight {
			return nil
		}
		stage := progressTransferStage(event, state.writeStatusEvents)
		if stage == "" {
			return nil
		}
		doc := markTransferProgress(base, stage, privacyOverheadFromClient(event.PrivacyTotal), now())
		if err := writer.WriteNetworkTransfer(ctx, doc); err != nil {
			return fmt.Errorf("write in-flight network transfer evidence: %w", err)
		}
		return nil
	}
}

func progressTransferStage(event protocolclient.ProgressEvent, writeStatusProgress bool) string {
	switch event.Stage {
	case protocolclient.ProgressStageStatus:
		if !writeStatusProgress || event.State == protocol.SessionStatePublished {
			return ""
		}
		return "status"
	case protocolclient.ProgressStageChunk:
		if !hasCurrentPayloadOverhead(privacyOverheadFromClient(event.PrivacyTotal)) {
			return ""
		}
		return "chunk"
	default:
		return ""
	}
}

func markTransferProgress(doc control.NetworkTransfer, stage string, overhead *control.NetworkTransferPrivacyOverhead, updatedAt time.Time) control.NetworkTransfer {
	doc.Attempts = append([]control.NetworkTransferAttempt(nil), doc.Attempts...)
	stamp := formatTime(updatedAt)
	doc.Status = control.NetworkTransferStarted
	doc.Stage = stage
	doc.UpdatedAt = stamp
	doc.ErrorCode = ""
	doc.Error = ""
	doc.PrivacyOverhead = overhead
	if len(doc.Attempts) == 0 {
		doc.Attempts = []control.NetworkTransferAttempt{{
			AttemptID: "attempt-1",
			StartedAt: doc.StartedAt,
		}}
	}
	last := len(doc.Attempts) - 1
	doc.Attempts[last].Stage = stage
	doc.Attempts[last].Status = control.NetworkTransferStarted
	doc.Attempts[last].EndedAt = ""
	doc.Attempts[last].ErrorCode = ""
	doc.Attempts[last].Error = ""
	return doc
}

type priorNetworkTransferState struct {
	exists bool
	doc    control.NetworkTransfer
}

func priorNetworkTransfer(ctx context.Context, writer ArtifactWriter, sessionID string) (priorNetworkTransferState, error) {
	reader, ok := writer.(NetworkTransferReader)
	if !ok {
		return priorNetworkTransferState{}, nil
	}
	doc, err := reader.ReadNetworkTransfer(ctx, sessionID)
	if err == nil {
		return priorNetworkTransferState{exists: true, doc: doc}, nil
	}
	if priorNetworkTransferNotFound(err) {
		return priorNetworkTransferState{}, nil
	}
	return priorNetworkTransferState{exists: true}, fmt.Errorf("%w: prior network transfer evidence unavailable: %v", ErrPayloadOverheadMissing, err)
}

func priorNetworkTransferNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
		return true
	}
	var remote *protocolclient.RemoteError
	return errors.As(err, &remote) && remote.StatusCode == http.StatusNotFound
}

func payloadUploadSkipped(result protocolclient.Result, overhead *control.NetworkTransferPrivacyOverhead) bool {
	if result.Bytes != 0 || result.Chunks != 0 {
		return false
	}
	if result.Begin.State != protocol.SessionStatePublished {
		return false
	}
	if overhead == nil {
		return true
	}
	return overhead.PaddedChunks == 0 && overhead.BatchFrames == 0 && overhead.BatchedChunks == 0
}

func mergeSuccessfulRetry(ctx context.Context, writer ArtifactWriter, current control.NetworkTransfer, currentOverhead *control.NetworkTransferPrivacyOverhead, endedAt time.Time, alreadyPublished bool) (control.NetworkTransfer, error) {
	merged, err := mergeRetryAttempt(ctx, writer, current, currentOverhead, alreadyPublished)
	if err != nil {
		return control.NetworkTransfer{}, err
	}
	return finishTransfer(merged, outcome{
		status: control.NetworkTransferPublished,
		stage:  "commit",
	}, endedAt, nil), nil
}

func mergeRetryAttempt(ctx context.Context, writer ArtifactWriter, current control.NetworkTransfer, currentOverhead *control.NetworkTransferPrivacyOverhead, alreadyPublished bool) (control.NetworkTransfer, error) {
	reader, ok := writer.(NetworkTransferReader)
	if !ok {
		return control.NetworkTransfer{}, fmt.Errorf("%w: artifact writer cannot read prior network transfer evidence", ErrPayloadOverheadMissing)
	}
	previous, err := reader.ReadNetworkTransfer(ctx, current.SessionID)
	if err != nil {
		return control.NetworkTransfer{}, fmt.Errorf("%w: prior network transfer evidence unavailable: %v", ErrPayloadOverheadMissing, err)
	}
	if alreadyPublished && previous.Status != control.NetworkTransferPublished {
		return control.NetworkTransfer{}, fmt.Errorf("%w: prior network transfer status is %q", ErrPayloadOverheadMissing, previous.Status)
	}
	if previous.PrivacyOverhead == nil || !hasPayloadOverhead(*previous.PrivacyOverhead) {
		return control.NetworkTransfer{}, fmt.Errorf("%w: prior network transfer has no payload overhead counters", ErrPayloadOverheadMissing)
	}
	if previous.SessionID != current.SessionID {
		return control.NetworkTransfer{}, fmt.Errorf("%w: prior network transfer session_id %q does not match retry session_id %q", ErrPayloadOverheadMissing, previous.SessionID, current.SessionID)
	}
	if previous.ProfileID != current.ProfileID ||
		previous.TargetID != current.TargetID ||
		previous.SourceDeviceID != current.SourceDeviceID ||
		previous.TargetDeviceID != current.TargetDeviceID ||
		previous.ProtocolVersion != current.ProtocolVersion ||
		previous.PrivacyPolicy != current.PrivacyPolicy {
		return control.NetworkTransfer{}, fmt.Errorf("%w: prior network transfer scope does not match retry request", ErrPayloadOverheadMissing)
	}
	if !alreadyPublished {
		previous.PrivacyOverhead = mergePrivacyOverhead(previous.PrivacyOverhead, currentOverhead)
	}
	return appendRetryAttempt(previous, current), nil
}

func hasPayloadOverhead(overhead control.NetworkTransferPrivacyOverhead) bool {
	return overhead.PaddedChunks > 0 || overhead.BatchFrames > 0 || overhead.BatchedChunks > 0
}

func hasCurrentPayloadOverhead(overhead *control.NetworkTransferPrivacyOverhead) bool {
	return overhead != nil && hasPayloadOverhead(*overhead)
}

func mergePrivacyOverhead(previous, current *control.NetworkTransferPrivacyOverhead) *control.NetworkTransferPrivacyOverhead {
	if previous == nil {
		return current
	}
	if current == nil {
		return previous
	}
	merged := *previous
	merged.FramePlainBytes += current.FramePlainBytes
	merged.FrameWireBytes += current.FrameWireBytes
	merged.PaddingBytes += current.PaddingBytes
	merged.PaddedChunks += current.PaddedChunks
	merged.BatchFrames += current.BatchFrames
	merged.BatchedChunks += current.BatchedChunks
	merged.JitteredRequests += current.JitteredRequests
	merged.JitterDelayMillis += current.JitterDelayMillis
	if current.PaddingBucketBytes > merged.PaddingBucketBytes {
		merged.PaddingBucketBytes = current.PaddingBucketBytes
	}
	if current.MaxBatchCount > merged.MaxBatchCount {
		merged.MaxBatchCount = current.MaxBatchCount
	}
	if current.MaxBatchPlainBytes > merged.MaxBatchPlainBytes {
		merged.MaxBatchPlainBytes = current.MaxBatchPlainBytes
	}
	if current.MaxJitterDelayMillis > merged.MaxJitterDelayMillis {
		merged.MaxJitterDelayMillis = current.MaxJitterDelayMillis
	}
	if current.JitterBudgetMillis > merged.JitterBudgetMillis {
		merged.JitterBudgetMillis = current.JitterBudgetMillis
	}
	return &merged
}

func requiresPriorPayloadEvidence(result protocolclient.Result, overhead *control.NetworkTransferPrivacyOverhead) bool {
	if result.Begin.State == protocol.SessionStatePublished && result.Bytes == 0 && result.Chunks == 0 {
		return true
	}
	if result.Bytes == 0 && result.Chunks == 0 && !hasCurrentPayloadOverhead(overhead) {
		for _, file := range resultResumeFrom(result) {
			if file.Complete {
				return true
			}
		}
	}
	for _, file := range resultResumeFrom(result) {
		if file.CommittedSize > 0 && !file.Complete {
			return true
		}
	}
	return false
}

func commitOnlyPayloadRetry(result protocolclient.Result) bool {
	if result.Bytes != 0 || result.Chunks != 0 {
		return false
	}
	for _, file := range resultResumeFrom(result) {
		if file.Complete {
			return true
		}
	}
	return false
}

func shouldMergeFailedRetry(result protocolclient.Result, failure outcome) bool {
	switch failure.status {
	case control.NetworkTransferFailed, control.NetworkTransferInterrupted, control.NetworkTransferPublishFailed:
	default:
		return false
	}
	if strings.TrimSpace(result.Begin.SessionID) == "" {
		return false
	}
	for _, file := range resultResumeFrom(result) {
		if file.CommittedSize > 0 || file.Complete {
			return true
		}
	}
	return false
}

func resultResumeFrom(result protocolclient.Result) []protocol.FileStatus {
	if len(result.ResumeFrom) > 0 {
		return result.ResumeFrom
	}
	return result.Begin.ResumeFrom
}

func appendRetryAttempt(previous, current control.NetworkTransfer) control.NetworkTransfer {
	if len(current.Attempts) == 0 {
		current.Attempts = []control.NetworkTransferAttempt{{
			AttemptID: "attempt-1",
			StartedAt: current.StartedAt,
			Stage:     "begin",
			Status:    control.NetworkTransferStarted,
		}}
	}
	next := current.Attempts[len(current.Attempts)-1]
	next.AttemptID = fmt.Sprintf("attempt-%d", len(previous.Attempts)+1)
	previous.Attempts = append(previous.Attempts, next)
	previous.UpdatedAt = current.UpdatedAt
	previous.Error = current.Error
	previous.ErrorCode = current.ErrorCode
	previous.Stage = current.Stage
	previous.Status = current.Status
	return previous
}

func newTransfer(req protocolclient.TransferRequest, startedAt time.Time) control.NetworkTransfer {
	stamp := formatTime(startedAt)
	attempt := control.NetworkTransferAttempt{
		AttemptID: "attempt-1",
		StartedAt: stamp,
		Stage:     "begin",
		Status:    control.NetworkTransferStarted,
	}
	return control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       req.SessionID,
		ProfileID:       req.ProfileID,
		TargetID:        req.TargetID,
		SourceDeviceID:  req.SourceDeviceID,
		TargetDeviceID:  req.TargetDeviceID,
		ProtocolVersion: protocol.Version,
		PrivacyPolicy:   req.PrivacyPolicy,
		Status:          control.NetworkTransferStarted,
		Stage:           "begin",
		StartedAt:       stamp,
		UpdatedAt:       stamp,
		Attempts:        []control.NetworkTransferAttempt{attempt},
	}
}

type outcome struct {
	status control.NetworkTransferStatus
	stage  string
	code   string
}

func finishTransfer(doc control.NetworkTransfer, out outcome, endedAt time.Time, cause error) control.NetworkTransfer {
	stamp := formatTime(endedAt)
	doc.Status = out.status
	doc.Stage = out.stage
	doc.UpdatedAt = stamp
	doc.ErrorCode = out.code
	if cause != nil {
		doc.Error = cause.Error()
	} else {
		doc.Error = ""
	}
	if len(doc.Attempts) == 0 {
		doc.Attempts = append(doc.Attempts, control.NetworkTransferAttempt{
			AttemptID: "attempt-1",
			StartedAt: doc.StartedAt,
		})
	}
	last := len(doc.Attempts) - 1
	doc.Attempts[last].EndedAt = stamp
	doc.Attempts[last].Stage = out.stage
	doc.Attempts[last].Status = out.status
	doc.Attempts[last].ErrorCode = out.code
	if cause != nil {
		doc.Attempts[last].Error = cause.Error()
	} else {
		doc.Attempts[last].Error = ""
	}
	return doc
}

func classifyError(err error) outcome {
	var remote *protocolclient.RemoteError
	switch {
	case errors.Is(err, protocolclient.ErrReceiverNeedsRepair):
		return outcome{status: control.NetworkTransferNeedsRepair, stage: "status", code: "receiver_needs_repair"}
	case errors.As(err, &remote) && remote.StatusCode == http.StatusForbidden:
		return outcome{status: control.NetworkTransferAuthRefused, stage: stageFromRemote(remote), code: "auth_refused"}
	case errors.As(err, &remote) && remote.Code == protocol.ErrorCodeIntegrity:
		return outcome{status: control.NetworkTransferNeedsRepair, stage: stageFromRemote(remote), code: "integrity_failure"}
	case errors.As(err, &remote) && remote.Path == "/v1/commit":
		return outcome{status: control.NetworkTransferPublishFailed, stage: "commit", code: "publish_failed"}
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return outcome{status: control.NetworkTransferInterrupted, stage: "transport", code: "interrupted"}
	default:
		return outcome{status: control.NetworkTransferFailed, stage: "transport", code: "transfer_failed"}
	}
}

func stageFromRemote(remote *protocolclient.RemoteError) string {
	switch remote.Path {
	case "/v1/sessions":
		return "begin"
	case "/v1/chunks":
		return "chunk"
	case "/v1/commit":
		return "commit"
	default:
		if strings.HasSuffix(remote.Path, "/status") {
			return "status"
		}
		return "transport"
	}
}

func warningArtifacts(sessionID string, now time.Time, warnings []audit.Record) []control.Warning {
	stamp := formatTime(now)
	docs := make([]control.Warning, 0, len(warnings))
	for i, warning := range warnings {
		doc := control.Warning{
			Version:               control.CurrentVersion,
			ID:                    fmt.Sprintf("%s-%03d-%s", sessionID, i+1, warning.ID),
			SessionID:             sessionID,
			Code:                  warning.Kind,
			Message:               warning.Reason,
			Severity:              string(warning.Severity),
			Paths:                 []string{warning.Path},
			TargetPath:            warning.TargetPath,
			Detected:              warning.Detected,
			SuggestedProfilePatch: warning.SuggestedProfilePatch,
			SuggestedConfig:       warning.SuggestedConfig,
			CreatedAt:             stamp,
		}
		docs = append(docs, doc)
	}
	return docs
}

func nowFunc(fn func() time.Time) func() time.Time {
	if fn != nil {
		return fn
	}
	return time.Now
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
