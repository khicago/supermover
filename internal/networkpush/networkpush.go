package networkpush

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/localpush"
	"github.com/khicago/supermover/internal/networkrun"
	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/protocolclient"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/tlsidentity"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/transport"
)

type Options struct {
	Profile   profile.Profile
	SessionID string
	Now       func() time.Time
}

type Result struct {
	SessionID       string
	Files           int
	Bytes           int64
	Chunks          int
	Warnings        int
	ResumeAuthority string
	ResumeOutcome   string
	ResumedBytes    int64
	TransferStatus  control.NetworkTransferStatus
	TransferStage   string
	TransferCode    string
	TransferError   string
}

func Run(ctx context.Context, opts Options) (Result, error) {
	prepared, err := prepare(ctx, opts, true)
	if err != nil {
		return Result{}, err
	}
	certificate, err := tlsidentity.Load(opts.Profile.Network.LocalTLSIdentity)
	if err != nil {
		return Result{}, fmt.Errorf("load local TLS identity: %w", err)
	}
	tlsConfig, err := transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{certificate},
		SourceDeviceID: prepared.trust.Receipt.SourceDeviceID,
		TargetDeviceID: prepared.trust.TargetDeviceID,
		Time:           opts.Now,
	})
	if err != nil {
		return Result{}, fmt.Errorf("build client TLS config: %w", err)
	}
	httpTransport := &http.Transport{TLSClientConfig: tlsConfig}
	defer httpTransport.CloseIdleConnections()
	httpClient := &http.Client{Transport: httpTransport}

	runResult, err := networkrun.Run(ctx, networkrun.Options{
		ArtifactWriter: networkrun.HTTPArtifactWriter{
			BaseURL: opts.Profile.Network.ReceiverURL,
			Doer:    httpClient,
		},
		ProfileSnapshot:      &prepared.profile,
		ProfilePrivacyPolicy: opts.Profile.PrivacyPolicy,
		Request:              prepared.request,
		Client: protocolclient.Client{
			BaseURL: opts.Profile.Network.ReceiverURL,
			Doer:    httpClient,
			Now:     opts.Now,
		},
		Now: opts.Now,
	})
	result := resultFromNetworkRun(prepared.request.SessionID, runResult)
	return result, err
}

func Preflight(ctx context.Context, opts Options) (Result, error) {
	prepared, err := prepare(ctx, opts, true)
	if err != nil {
		return Result{}, err
	}
	begin, warnings, err := protocolclient.BuildBeginRequest(prepared.request)
	if err != nil {
		return Result{}, err
	}
	files, bytes := manifestStats(begin.Manifest.Entries)
	return Result{
		SessionID:       prepared.request.SessionID,
		Files:           files,
		Bytes:           bytes,
		Warnings:        len(warnings),
		ResumeAuthority: "not_attempted",
		ResumeOutcome:   "not_attempted",
		TransferStage:   "preflight",
		TransferStatus:  "",
	}, nil
}

type preparedRun struct {
	trust   pairing.TrustState
	request protocolclient.TransferRequest
	profile control.ProfileSnapshot
}

func prepare(ctx context.Context, opts Options, validateTLSIdentity bool) (preparedRun, error) {
	if ctx == nil {
		return preparedRun{}, errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return preparedRun{}, err
	}
	if err := opts.Profile.ValidateNetworkClientMaterial(); err != nil {
		return preparedRun{}, fmt.Errorf("profile network client material: %w", err)
	}
	if err := ValidateProfileForNetworkPush(opts.Profile); err != nil {
		return preparedRun{}, err
	}
	trust, err := pairing.ValidateProfileTrust(opts.Profile)
	if err != nil {
		return preparedRun{}, fmt.Errorf("profile trust: %w", err)
	}
	if validateTLSIdentity {
		if err := tlsidentity.ValidatePinned(opts.Profile.Network.LocalTLSIdentity, trust.Receipt.SourceDeviceID, opts.Now); err != nil {
			return preparedRun{}, fmt.Errorf("validate local TLS identity files: %w", err)
		}
	}
	now := nowFunc(opts.Now)
	createdAt := now()
	sessionID := strings.TrimSpace(opts.SessionID)
	if sessionID == "" {
		sessionID = generatedSessionID(createdAt)
	}
	if err := transaction.ValidateSessionID(sessionID); err != nil {
		return preparedRun{}, err
	}
	root := opts.Profile.Roots[0]
	scanned, err := scan.Scan(root.Path)
	if err != nil {
		return preparedRun{}, fmt.Errorf("scan source root: %w", err)
	}
	privacyPolicy, err := transportPrivacyPolicyFromProfile(opts.Profile.PrivacyPolicy)
	if err != nil {
		return preparedRun{}, err
	}
	request := protocolclient.TransferRequest{
		SourceRoot:     root.Path,
		Scan:           scanned,
		SessionID:      sessionID,
		ManifestID:     sessionID,
		ProfileID:      opts.Profile.ProfileID,
		TargetID:       opts.Profile.Target.TargetID,
		SourceDeviceID: trust.Receipt.SourceDeviceID,
		TargetDeviceID: trust.TargetDeviceID,
		PrivacyPolicy:  privacyPolicy,
		RootID:         root.ID,
		CreatedAt:      createdAt,
	}
	if _, _, err := protocolclient.BuildBeginRequest(request); err != nil {
		return preparedRun{}, err
	}
	snapshot, err := profileSnapshot(opts.Profile, sessionID, createdAt)
	if err != nil {
		return preparedRun{}, err
	}
	return preparedRun{
		trust:   trust,
		request: request,
		profile: snapshot,
	}, nil
}

func resultFromNetworkRun(sessionID string, runResult networkrun.Result) Result {
	if strings.TrimSpace(runResult.ClientResult.SessionID) != "" {
		sessionID = runResult.ClientResult.SessionID
	}
	resumeAuthority, resumeOutcome, resumedBytes := resumeEvidence(runResult.ClientResult, runResult.Transfer)
	return Result{
		SessionID:       sessionID,
		Files:           runResult.ClientResult.Files,
		Bytes:           runResult.ClientResult.Bytes,
		Chunks:          runResult.ClientResult.Chunks,
		Warnings:        len(runResult.ClientResult.Warnings),
		ResumeAuthority: resumeAuthority,
		ResumeOutcome:   resumeOutcome,
		ResumedBytes:    resumedBytes,
		TransferStatus:  runResult.Transfer.Status,
		TransferStage:   runResult.Transfer.Stage,
		TransferCode:    runResult.Transfer.ErrorCode,
		TransferError:   runResult.Transfer.Error,
	}
}

func resumeEvidence(result protocolclient.Result, transfer control.NetworkTransfer) (string, string, int64) {
	if strings.TrimSpace(result.SessionID) == "" {
		return "not_attempted", "not_attempted", 0
	}
	if strings.TrimSpace(result.Begin.SessionID) == "" {
		return "not_available", "begin_failed", 0
	}
	if transfer.Status != "" && transfer.Status != control.NetworkTransferPublished {
		return "receiver_status", "blocked", 0
	}
	if result.Begin.State == protocol.SessionStatePublished && result.Bytes == 0 && result.Chunks == 0 {
		return "receiver_status", "published_retry", 0
	}
	resumeFrom := resultResumeFrom(result)
	for _, file := range resumeFrom {
		if file.CommittedSize > 0 && !file.Complete {
			return "receiver_status", "resumed", result.Bytes
		}
	}
	for _, file := range resumeFrom {
		if file.Complete {
			return "receiver_status", "receiver_status", 0
		}
	}
	if len(result.Begin.ResumeFrom) > 0 {
		return "receiver_status", "receiver_status", 0
	}
	return "receiver_status", "fresh", 0
}

func resultResumeFrom(result protocolclient.Result) []protocol.FileStatus {
	if len(result.ResumeFrom) > 0 {
		return result.ResumeFrom
	}
	return result.Begin.ResumeFrom
}

func ValidateProfileForNetworkPush(p profile.Profile) error {
	if len(p.Roots) != 1 {
		return fmt.Errorf("network push requires exactly one root for now")
	}
	if err := localpush.ValidateSupportedRules(p); err != nil {
		return err
	}
	if p.Consistency != profile.ConsistencyStrict {
		return fmt.Errorf("consistency=%q is not implemented in network push; only strict is supported", p.Consistency)
	}
	if p.DeletePolicy.Mode == profile.DeleteModePrune || p.DeletePolicy.AllowPhysicalPrune {
		return fmt.Errorf("physical prune is not implemented in network push; use delete_policy.mode=record or ignore")
	}
	if p.PrivacyPolicy.Mode != profile.PrivacyModePlaintext {
		return fmt.Errorf("privacy_policy.mode=%q is not implemented in network push; receiver writes plaintext files", p.PrivacyPolicy.Mode)
	}
	if p.PrivacyPolicy.TrafficLevel != int(transport.PrivacyLevel2) {
		return fmt.Errorf("privacy_policy.traffic_level=%d is not implemented in network push; only traffic level 2 is supported", p.PrivacyPolicy.TrafficLevel)
	}
	if !p.PrivacyPolicy.AllowHiddenFiles {
		return fmt.Errorf("privacy_policy.allow_hidden_files=false is not implemented in network push; hidden files are always included")
	}
	if !p.PrivacyPolicy.AllowSensitiveFilenames {
		return fmt.Errorf("privacy_policy.allow_sensitive_filenames=false is not implemented in network push; sensitive filenames are always included")
	}
	if p.MetadataPolicy.PreserveExtendedAttr {
		return fmt.Errorf("metadata_policy.preserve_extended_attr=true is not implemented in network push")
	}
	if !p.MetadataPolicy.PreservePermissions {
		return fmt.Errorf("metadata_policy.preserve_permissions=false is not implemented in network push; permissions are always preserved")
	}
	if !p.MetadataPolicy.PreserveModTime {
		return fmt.Errorf("metadata_policy.preserve_mod_time=false is not implemented in network push; modification times are always preserved")
	}
	if p.MetadataPolicy.Mode != profile.MetadataModeBasic {
		return fmt.Errorf("metadata_policy.mode=%q is not implemented in network push; only basic is supported", p.MetadataPolicy.Mode)
	}
	return nil
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

func profileSnapshot(p profile.Profile, sessionID string, capturedAt time.Time) (control.ProfileSnapshot, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return control.ProfileSnapshot{}, fmt.Errorf("marshal profile snapshot: %w", err)
	}
	return control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-" + sessionID,
		ProfileID:  p.ProfileID,
		SessionID:  sessionID,
		CapturedAt: capturedAt.UTC().Format(time.RFC3339Nano),
		Profile:    payload,
	}, nil
}

func generatedSessionID(now time.Time) string {
	return "session-" + now.UTC().Format("20060102T150405Z")
}

func manifestStats(entries []protocol.ManifestEntry) (int, int64) {
	var files int
	var bytes int64
	for _, entry := range entries {
		if entry.Kind == protocol.FileKindFile {
			files++
			bytes += entry.Size
		}
	}
	return files, bytes
}

func nowFunc(fn func() time.Time) func() time.Time {
	if fn != nil {
		return func() time.Time { return fn().UTC() }
	}
	return func() time.Time { return time.Now().UTC() }
}
