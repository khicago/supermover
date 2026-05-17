package protocol

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/transport"
)

func TestBeginSessionRequestValidate(t *testing.T) {
	valid := validBeginSessionRequest()
	tests := []struct {
		name    string
		req     BeginSessionRequest
		wantErr bool
	}{
		{name: "valid", req: valid},
		{name: "wrong protocol", req: withProtocolVersion(valid, "supermover/2"), wantErr: true},
		{name: "dot-only session id", req: withSessionID(valid, "."), wantErr: true},
		{name: "missing target id", req: withTargetID(valid, ""), wantErr: true},
		{name: "unsafe target id", req: withTargetID(valid, "target id"), wantErr: true},
		{name: "absolute path", req: withManifestEntry(valid, ManifestEntry{Path: "/abs", Kind: FileKindFile, Size: 1, Digest: validDigest()}), wantErr: true},
		{name: "windows absolute volume path", req: withManifestEntry(valid, ManifestEntry{Path: "C:/abs", Kind: FileKindFile, Size: 1, Digest: validDigest()}), wantErr: true},
		{name: "windows drive relative path", req: withManifestEntry(valid, ManifestEntry{Path: "C:abs", Kind: FileKindFile, Size: 1, Digest: validDigest()}), wantErr: true},
		{name: "windows unc path", req: withManifestEntry(valid, ManifestEntry{Path: "//server/share", Kind: FileKindFile, Size: 1, Digest: validDigest()}), wantErr: true},
		{name: "backslash path", req: withManifestEntry(valid, ManifestEntry{Path: `dir\file.bin`, Kind: FileKindFile, Size: 1, Digest: validDigest()}), wantErr: true},
		{name: "traversal path", req: withManifestEntry(valid, ManifestEntry{Path: "dir/../file.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()}), wantErr: true},
		{name: "unsafe target path", req: withManifestEntry(valid, ManifestEntry{Path: "file.bin", TargetPath: "../escape.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()}), wantErr: true},
		{name: "valid hidden file", req: withManifestEntry(valid, ManifestEntry{Path: ".env", Kind: FileKindFile, Size: 1, Digest: validDigest()})},
		{name: "valid hidden directory file", req: withManifestEntry(valid, ManifestEntry{Path: ".config/settings.json", Kind: FileKindFile, Size: 1, Digest: validDigest()})},
		{name: "reserved target path", req: withManifestEntry(valid, ManifestEntry{Path: "file.bin", TargetPath: ".supermover/sessions/fake/receipt.json", Kind: FileKindFile, Size: 1, Digest: validDigest()}), wantErr: true},
		{name: "reserved path without target override", req: withManifestEntry(valid, ManifestEntry{Path: ".Supermover/sessions/fake/receipt.json", Kind: FileKindFile, Size: 1, Digest: validDigest()}), wantErr: true},
		{name: "missing file digest", req: withManifestEntry(valid, ManifestEntry{Path: "file.bin", Kind: FileKindFile, Size: 1}), wantErr: true},
		{name: "zero-byte file empty digest", req: withManifestEntry(valid, ManifestEntry{Path: "empty.bin", Kind: FileKindFile, Size: 0, Digest: EmptySHA256Digest})},
		{name: "zero-byte file non-empty digest", req: withManifestEntry(valid, ManifestEntry{Path: "empty.bin", Kind: FileKindFile, Size: 0, Digest: validDigest()}), wantErr: true},
		{name: "invalid mode", req: withManifestEntry(valid, ManifestEntry{Path: "file.bin", Kind: FileKindFile, Mode: 0o1000, Size: 1, Digest: validDigest()}), wantErr: true},
		{name: "unsafe symlink target", req: withManifestEntry(valid, ManifestEntry{Path: "link", Kind: FileKindSymlink, SymlinkTarget: "../outside"}), wantErr: true},
		{name: "reserved symlink target", req: withManifestEntry(valid, ManifestEntry{Path: "link", Kind: FileKindSymlink, SymlinkTarget: ".supermover/sessions/fake/receipt.json"}), wantErr: true},
		{name: "duplicate path", req: withManifest(valid, TransferManifest{ID: "manifest1", Entries: []ManifestEntry{
			{Path: "file.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()},
			{Path: "file.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()},
		}}), wantErr: true},
		{name: "duplicate target path", req: withManifest(valid, TransferManifest{ID: "manifest1", Entries: []ManifestEntry{
			{Path: "a.bin", TargetPath: "same.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()},
			{Path: "b.bin", TargetPath: "same.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()},
		}}), wantErr: true},
		{name: "file below previous symlink", req: withManifest(valid, TransferManifest{ID: "manifest1", Entries: []ManifestEntry{
			{Path: "linkdir", Kind: FileKindSymlink, SymlinkTarget: "outside"},
			{Path: "linkdir/file.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()},
		}}), wantErr: true},
		{name: "symlink above previous file", req: withManifest(valid, TransferManifest{ID: "manifest1", Entries: []ManifestEntry{
			{Path: "linkdir/file.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()},
			{Path: "linkdir", Kind: FileKindSymlink, SymlinkTarget: "outside"},
		}}), wantErr: true},
		{name: "file below symlink target path", req: withManifest(valid, TransferManifest{ID: "manifest1", Entries: []ManifestEntry{
			{Path: "linkdir", TargetPath: "published-link", Kind: FileKindSymlink, SymlinkTarget: "outside"},
			{Path: "docs/file.bin", TargetPath: "published-link/file.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()},
		}}), wantErr: true},
		{name: "sibling prefix below symlink allowed", req: withManifest(valid, TransferManifest{ID: "manifest1", Entries: []ManifestEntry{
			{Path: "linkdir", Kind: FileKindSymlink, SymlinkTarget: "outside"},
			{Path: "linkdir2/file.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()},
		}})},
		{name: "manifest total bytes too large", req: withManifestEntry(valid, ManifestEntry{Path: "huge.bin", Kind: FileKindFile, Size: MaxTotalDeclaredBytes + 1, Digest: validDigest()}), wantErr: true},
		{name: "level 2 padding bucket exceeds protocol max", req: withPrivacyPolicy(valid, privacyPolicyWith(func(p transport.PrivacyPolicy) transport.PrivacyPolicy {
			p.PaddingBucket = MaxPaddingBucketBytes + 1
			return p
		})), wantErr: true},
		{name: "level 2 batch bytes exceeds protocol max", req: withPrivacyPolicy(valid, privacyPolicyWith(func(p transport.PrivacyPolicy) transport.PrivacyPolicy {
			p.BatchMaxBytes = MaxBatchPlainBodyBytes + 1
			return p
		})), wantErr: true},
		{name: "level 2 batch count exceeds protocol max", req: withPrivacyPolicy(valid, privacyPolicyWith(func(p transport.PrivacyPolicy) transport.PrivacyPolicy {
			p.BatchMaxCount = MaxBatchChunks + 1
			return p
		})), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("BeginSessionRequest.Validate(%+v) error = %v, want error presence = %t", tt.req, err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrValidation) {
				t.Errorf("BeginSessionRequest.Validate(%+v) error = %v, want ErrValidation", tt.req, err)
			}
		})
	}
}

func privacyPolicyWith(edit func(transport.PrivacyPolicy) transport.PrivacyPolicy) transport.PrivacyPolicy {
	return edit(transport.DefaultPrivacyPolicy(transport.PrivacyLevel2))
}

func TestChunkUploadRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     ChunkUploadRequest
		wantErr bool
	}{
		{name: "valid", req: ChunkUploadRequest{SessionID: "session-1", Path: "dir/file.bin", Offset: 0, Data: []byte("abc")}},
		{name: "zero-byte completion without digest", req: ChunkUploadRequest{SessionID: "session-1", Path: "dir/empty.bin", Offset: 0, Data: []byte{}, Final: true}},
		{name: "zero-byte completion with empty digest", req: ChunkUploadRequest{SessionID: "session-1", Path: "dir/empty.bin", Offset: 0, Data: []byte{}, Digest: EmptySHA256Digest, Final: true}},
		{name: "dot-only session id", req: ChunkUploadRequest{SessionID: ".", Path: "dir/file.bin", Offset: 0, Data: []byte("abc")}, wantErr: true},
		{name: "missing data", req: ChunkUploadRequest{SessionID: "session-1", Path: "dir/file.bin"}, wantErr: true},
		{name: "zero-byte completion must be final", req: ChunkUploadRequest{SessionID: "session-1", Path: "dir/empty.bin", Offset: 0, Data: []byte{}}, wantErr: true},
		{name: "zero-byte completion offset must be zero", req: ChunkUploadRequest{SessionID: "session-1", Path: "dir/empty.bin", Offset: 1, Data: []byte{}, Final: true}, wantErr: true},
		{name: "zero-byte completion rejects non-empty digest", req: ChunkUploadRequest{SessionID: "session-1", Path: "dir/empty.bin", Offset: 0, Data: []byte{}, Digest: validDigest(), Final: true}, wantErr: true},
		{name: "chunk too large", req: ChunkUploadRequest{SessionID: "session-1", Path: "dir/file.bin", Data: make([]byte, MaxChunkBytes+1)}, wantErr: true},
		{name: "negative offset", req: ChunkUploadRequest{SessionID: "session-1", Path: "dir/file.bin", Offset: -1, Data: []byte("abc")}, wantErr: true},
		{name: "bad digest", req: ChunkUploadRequest{SessionID: "session-1", Path: "dir/file.bin", Data: []byte("abc"), Digest: "md5:abc"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("ChunkUploadRequest.Validate(%+v) error = %v, want error presence = %t", tt.req, err, tt.wantErr)
			}
		})
	}
}

func TestChunkBatchUploadRequestValidate(t *testing.T) {
	validChunk := ChunkUploadRequest{SessionID: "session-1", Path: "dir/file.bin", Offset: 0, Data: []byte("abc")}
	tooManyChunks := make([]ChunkUploadRequest, MaxBatchChunks+1)
	for i := range tooManyChunks {
		tooManyChunks[i] = ChunkUploadRequest{SessionID: "session-1", Path: "dir/file.bin", Offset: int64(i), Data: []byte("a")}
	}
	tests := []struct {
		name    string
		req     ChunkBatchUploadRequest
		wantErr bool
	}{
		{
			name: "valid",
			req: ChunkBatchUploadRequest{
				SessionID: "session-1",
				Chunks:    []ChunkUploadRequest{validChunk},
			},
		},
		{
			name: "empty batch",
			req: ChunkBatchUploadRequest{
				SessionID: "session-1",
			},
			wantErr: true,
		},
		{
			name: "too many chunks",
			req: ChunkBatchUploadRequest{
				SessionID: "session-1",
				Chunks:    tooManyChunks,
			},
			wantErr: true,
		},
		{
			name: "mismatched chunk session",
			req: ChunkBatchUploadRequest{
				SessionID: "session-1",
				Chunks: []ChunkUploadRequest{
					{SessionID: "session-2", Path: "dir/file.bin", Offset: 0, Data: []byte("abc")},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid nested chunk",
			req: ChunkBatchUploadRequest{
				SessionID: "session-1",
				Chunks: []ChunkUploadRequest{
					{SessionID: "session-1", Path: "../escape.bin", Offset: 0, Data: []byte("abc")},
				},
			},
			wantErr: true,
		},
		{
			name: "oversized count uses max batch chunks",
			req: ChunkBatchUploadRequest{
				SessionID: "session-1",
				Chunks:    make([]ChunkUploadRequest, MaxBatchChunks+1),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("ChunkBatchUploadRequest.Validate(%+v) error = %v, want error presence = %t", tt.req, err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrValidation) {
				t.Errorf("ChunkBatchUploadRequest.Validate(%+v) error = %v, want ErrValidation", tt.req, err)
			}
		})
	}
}

func TestCommitSessionRequestValidateRejectsDotOnlySessionID(t *testing.T) {
	err := (CommitSessionRequest{SessionID: ".", EndedAt: time.Date(2026, 5, 16, 8, 5, 0, 0, time.UTC)}).Validate()
	if err == nil {
		t.Fatalf("CommitSessionRequest.Validate() error = nil, want unsafe session id")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("CommitSessionRequest.Validate() error = %v, want ErrValidation", err)
	}
}

func TestTransferManifestValidateLargeSymlinkSet(t *testing.T) {
	entries := make([]ManifestEntry, 0, 5_000)
	for i := 0; i < 2_500; i++ {
		entries = append(entries, ManifestEntry{Path: "links/link" + strconv.Itoa(i), Kind: FileKindSymlink, SymlinkTarget: "targets/target" + strconv.Itoa(i)})
		entries = append(entries, ManifestEntry{Path: "files/file" + strconv.Itoa(i), Kind: FileKindFile, Size: 1, Digest: validDigest()})
	}
	manifest := TransferManifest{ID: "manifest1", Entries: entries}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("TransferManifest.Validate(large symlink set) error = %v, want nil", err)
	}
}

func validBeginSessionRequest() BeginSessionRequest {
	return BeginSessionRequest{
		ProtocolVersion: Version,
		SessionID:       "session-1",
		ProfileID:       "profile.default",
		TargetID:        "local:profile.default",
		SourceDeviceID:  "sha256:abcdef0123456789",
		TargetDeviceID:  "sha256:0123456789abcdef",
		RootID:          "root1",
		CreatedAt:       time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC),
		Manifest: TransferManifest{
			ID: "manifest1",
			Entries: []ManifestEntry{
				{Path: "file.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()},
			},
		},
	}
}

func validDigest() string {
	return "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
}

func withProtocolVersion(req BeginSessionRequest, version string) BeginSessionRequest {
	req.ProtocolVersion = version
	return req
}

func withSessionID(req BeginSessionRequest, sessionID string) BeginSessionRequest {
	req.SessionID = sessionID
	return req
}

func withTargetID(req BeginSessionRequest, targetID string) BeginSessionRequest {
	req.TargetID = targetID
	return req
}

func withManifestEntry(req BeginSessionRequest, entry ManifestEntry) BeginSessionRequest {
	req.Manifest.Entries = []ManifestEntry{entry}
	return req
}

func withManifest(req BeginSessionRequest, manifest TransferManifest) BeginSessionRequest {
	req.Manifest = manifest
	return req
}

func withPrivacyPolicy(req BeginSessionRequest, policy transport.PrivacyPolicy) BeginSessionRequest {
	req.PrivacyPolicy = policy
	return req
}
