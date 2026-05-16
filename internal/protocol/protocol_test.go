package protocol

import (
	"errors"
	"testing"
	"time"
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
		{name: "absolute path", req: withManifestEntry(valid, ManifestEntry{Path: "/abs", Kind: FileKindFile, Size: 1, Digest: validDigest()}), wantErr: true},
		{name: "unsafe target path", req: withManifestEntry(valid, ManifestEntry{Path: "file.bin", TargetPath: "../escape.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()}), wantErr: true},
		{name: "missing file digest", req: withManifestEntry(valid, ManifestEntry{Path: "file.bin", Kind: FileKindFile, Size: 1}), wantErr: true},
		{name: "duplicate path", req: withManifest(valid, TransferManifest{ID: "manifest1", Entries: []ManifestEntry{
			{Path: "file.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()},
			{Path: "file.bin", Kind: FileKindFile, Size: 1, Digest: validDigest()},
		}}), wantErr: true},
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

func TestChunkUploadRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     ChunkUploadRequest
		wantErr bool
	}{
		{name: "valid", req: ChunkUploadRequest{SessionID: "session-1", Path: "dir/file.bin", Offset: 0, Data: []byte("abc")}},
		{name: "missing data", req: ChunkUploadRequest{SessionID: "session-1", Path: "dir/file.bin"}, wantErr: true},
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

func validBeginSessionRequest() BeginSessionRequest {
	return BeginSessionRequest{
		ProtocolVersion: Version,
		SessionID:       "session-1",
		ProfileID:       "profile.default",
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

func withManifestEntry(req BeginSessionRequest, entry ManifestEntry) BeginSessionRequest {
	req.Manifest.Entries = []ManifestEntry{entry}
	return req
}

func withManifest(req BeginSessionRequest, manifest TransferManifest) BeginSessionRequest {
	req.Manifest = manifest
	return req
}
