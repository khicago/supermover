package padding

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

func TestPaddedLen(t *testing.T) {
	cfg := Config{BucketBytes: 32, MaxFrameBytes: 128}

	tests := []struct {
		name        string
		plainLen    int
		wantWire    int
		wantPadding int
	}{
		{name: "zero", plainLen: 0, wantWire: 32, wantPadding: 18},
		{name: "one", plainLen: 1, wantWire: 32, wantPadding: 17},
		{name: "bucket minus one", plainLen: 31, wantWire: 64, wantPadding: 19},
		{name: "bucket", plainLen: 32, wantWire: 64, wantPadding: 18},
		{name: "bucket plus one", plainLen: 33, wantWire: 64, wantPadding: 17},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wireLen, paddingLen, err := PaddedLen(tt.plainLen, cfg)
			if err != nil {
				t.Fatalf("PaddedLen() error = %v", err)
			}
			if wireLen != tt.wantWire || paddingLen != tt.wantPadding {
				t.Fatalf("PaddedLen() = (%d, %d), want (%d, %d)", wireLen, paddingLen, tt.wantWire, tt.wantPadding)
			}
		})
	}
}

func TestPadUnpad(t *testing.T) {
	cfg := Config{BucketBytes: 32, MaxFrameBytes: 128}

	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "zero", payload: []byte{}},
		{name: "one", payload: []byte("a")},
		{name: "bucket minus one", payload: bytes.Repeat([]byte("b"), 31)},
		{name: "bucket", payload: bytes.Repeat([]byte("c"), 32)},
		{name: "bucket plus one", payload: bytes.Repeat([]byte("d"), 33)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire, padStats, err := Pad(tt.payload, cfg)
			if err != nil {
				t.Fatalf("Pad() error = %v", err)
			}
			wireAgain, statsAgain, err := Pad(tt.payload, cfg)
			if err != nil {
				t.Fatalf("Pad() second call error = %v", err)
			}
			if !bytes.Equal(wire, wireAgain) {
				t.Fatal("Pad() output is not deterministic")
			}
			if padStats != statsAgain {
				t.Fatalf("Pad() stats = %+v, second stats = %+v", padStats, statsAgain)
			}
			if len(wire)%cfg.BucketBytes != 0 {
				t.Fatalf("wire length %d is not bucket-aligned to %d", len(wire), cfg.BucketBytes)
			}

			gotPayload, unpadStats, err := Unpad(wire, cfg)
			if err != nil {
				t.Fatalf("Unpad() error = %v", err)
			}
			if !bytes.Equal(gotPayload, tt.payload) {
				t.Fatalf("Unpad() payload = %q, want %q", gotPayload, tt.payload)
			}
			if unpadStats != padStats {
				t.Fatalf("Unpad() stats = %+v, want %+v", unpadStats, padStats)
			}
		})
	}
}

func TestValidateRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "zero bucket", cfg: Config{BucketBytes: 0, MaxFrameBytes: 128}},
		{name: "negative bucket", cfg: Config{BucketBytes: -1, MaxFrameBytes: 128}},
		{name: "bucket smaller than header", cfg: Config{BucketBytes: headerLen - 1, MaxFrameBytes: 128}},
		{name: "zero max", cfg: Config{BucketBytes: 32, MaxFrameBytes: 0}},
		{name: "negative max", cfg: Config{BucketBytes: 32, MaxFrameBytes: -1}},
		{name: "max smaller than bucket", cfg: Config{BucketBytes: 32, MaxFrameBytes: 31}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.cfg); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestPaddedLenRejectsOverflowAndMaxFrame(t *testing.T) {
	tests := []struct {
		name     string
		plainLen int
		cfg      Config
	}{
		{name: "negative plain length", plainLen: -1, cfg: Config{BucketBytes: 32, MaxFrameBytes: 128}},
		{name: "plain length overflows header addition", plainLen: math.MaxInt - headerLen + 1, cfg: Config{BucketBytes: 32, MaxFrameBytes: math.MaxInt}},
		{name: "rounded frame exceeds max", plainLen: 33, cfg: Config{BucketBytes: 32, MaxFrameBytes: 63}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := PaddedLen(tt.plainLen, tt.cfg)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("PaddedLen() error = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestUnpadRejectsMalformedFrames(t *testing.T) {
	cfg := Config{BucketBytes: 32, MaxFrameBytes: 128}
	good, _, err := Pad([]byte("payload"), cfg)
	if err != nil {
		t.Fatalf("Pad() error = %v", err)
	}

	tests := []struct {
		name  string
		frame func() []byte
	}{
		{
			name: "bad magic",
			frame: func() []byte {
				wire := clone(good)
				wire[0] = 'X'
				return wire
			},
		},
		{
			name: "bad version",
			frame: func() []byte {
				wire := clone(good)
				wire[len(magic)] = version + 1
				return wire
			},
		},
		{
			name: "truncated header",
			frame: func() []byte {
				return clone(good[:headerLen-1])
			},
		},
		{
			name: "truncated body",
			frame: func() []byte {
				return clone(good[:len(good)-1])
			},
		},
		{
			name: "plain length outside frame",
			frame: func() []byte {
				wire := clone(good)
				wire[headerLen-1] = byte(len(wire))
				return wire
			},
		},
		{
			name: "length padding does not fit canonical bucket",
			frame: func() []byte {
				wire := clone(good)
				wire[headerLen-1] = 1
				return wire
			},
		},
		{
			name: "non-zero padding",
			frame: func() []byte {
				wire := clone(good)
				wire[len(wire)-1] = 1
				return wire
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Unpad(tt.frame(), cfg)
			if !errors.Is(err, ErrMalformedFrame) {
				t.Fatalf("Unpad() error = %v, want ErrMalformedFrame", err)
			}
		})
	}
}

func TestUnpadRejectsFrameOverMax(t *testing.T) {
	padCfg := Config{BucketBytes: 32, MaxFrameBytes: 128}
	wire, _, err := Pad(bytes.Repeat([]byte("x"), 50), padCfg)
	if err != nil {
		t.Fatalf("Pad() error = %v", err)
	}

	unpadCfg := Config{BucketBytes: 32, MaxFrameBytes: 32}
	_, _, err = Unpad(wire, unpadCfg)
	if !errors.Is(err, ErrMalformedFrame) {
		t.Fatalf("Unpad() error = %v, want ErrMalformedFrame", err)
	}
}

func clone(in []byte) []byte {
	out := make([]byte, len(in))
	copy(out, in)
	return out
}
