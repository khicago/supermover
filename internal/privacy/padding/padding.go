package padding

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const (
	magic        = "SMPAD"
	version byte = 1

	headerLen = len(magic) + 1 + 8
)

var (
	ErrInvalidConfig  = errors.New("invalid padding config")
	ErrMalformedFrame = errors.New("malformed padding frame")
)

type Config struct {
	BucketBytes   int
	MaxFrameBytes int
}

type Stats struct {
	PlainBytes   int
	WireBytes    int
	PaddingBytes int
	BucketBytes  int
}

func Validate(cfg Config) error {
	if cfg.BucketBytes <= 0 {
		return fmt.Errorf("%w: bucket bytes must be positive", ErrInvalidConfig)
	}
	if cfg.MaxFrameBytes <= 0 {
		return fmt.Errorf("%w: max frame bytes must be positive", ErrInvalidConfig)
	}
	if cfg.BucketBytes < headerLen {
		return fmt.Errorf("%w: bucket bytes must fit %d byte header", ErrInvalidConfig, headerLen)
	}
	if cfg.MaxFrameBytes < cfg.BucketBytes {
		return fmt.Errorf("%w: max frame bytes must be at least bucket bytes", ErrInvalidConfig)
	}
	return nil
}

func PaddedLen(plainLen int, cfg Config) (wireLen int, paddingLen int, err error) {
	if err := Validate(cfg); err != nil {
		return 0, 0, err
	}
	if plainLen < 0 {
		return 0, 0, fmt.Errorf("%w: plain length must be non-negative", ErrInvalidConfig)
	}
	if plainLen > math.MaxInt-headerLen {
		return 0, 0, fmt.Errorf("%w: plain length overflows frame length", ErrInvalidConfig)
	}

	needed := headerLen + plainLen
	wireLen, err = roundUp(needed, cfg.BucketBytes)
	if err != nil {
		return 0, 0, err
	}
	if wireLen > cfg.MaxFrameBytes {
		return 0, 0, fmt.Errorf("%w: frame length %d exceeds max %d", ErrInvalidConfig, wireLen, cfg.MaxFrameBytes)
	}
	return wireLen, wireLen - needed, nil
}

func Pad(payload []byte, cfg Config) ([]byte, Stats, error) {
	wireLen, paddingLen, err := PaddedLen(len(payload), cfg)
	if err != nil {
		return nil, Stats{}, err
	}

	wire := make([]byte, wireLen)
	copy(wire, magic)
	wire[len(magic)] = version
	binary.BigEndian.PutUint64(wire[len(magic)+1:headerLen], uint64(len(payload)))
	copy(wire[headerLen:], payload)

	return wire, stats(len(payload), wireLen, paddingLen, cfg.BucketBytes), nil
}

func Unpad(wire []byte, cfg Config) ([]byte, Stats, error) {
	if err := Validate(cfg); err != nil {
		return nil, Stats{}, err
	}
	if len(wire) < headerLen {
		return nil, Stats{}, fmt.Errorf("%w: truncated header", ErrMalformedFrame)
	}
	if len(wire) > cfg.MaxFrameBytes {
		return nil, Stats{}, fmt.Errorf("%w: frame length %d exceeds max %d", ErrMalformedFrame, len(wire), cfg.MaxFrameBytes)
	}
	if len(wire)%cfg.BucketBytes != 0 {
		return nil, Stats{}, fmt.Errorf("%w: frame length %d does not fit bucket %d", ErrMalformedFrame, len(wire), cfg.BucketBytes)
	}
	if string(wire[:len(magic)]) != magic {
		return nil, Stats{}, fmt.Errorf("%w: bad magic", ErrMalformedFrame)
	}
	if wire[len(magic)] != version {
		return nil, Stats{}, fmt.Errorf("%w: bad version", ErrMalformedFrame)
	}

	plainLen64 := binary.BigEndian.Uint64(wire[len(magic)+1 : headerLen])
	if plainLen64 > uint64(math.MaxInt) {
		return nil, Stats{}, fmt.Errorf("%w: plain length exceeds int", ErrMalformedFrame)
	}
	plainLen := int(plainLen64)
	if plainLen > len(wire)-headerLen {
		return nil, Stats{}, fmt.Errorf("%w: plain length outside frame", ErrMalformedFrame)
	}

	expectedWireLen, paddingLen, err := PaddedLen(plainLen, cfg)
	if err != nil {
		return nil, Stats{}, err
	}
	if expectedWireLen != len(wire) {
		return nil, Stats{}, fmt.Errorf("%w: frame length %d does not match canonical bucket length %d", ErrMalformedFrame, len(wire), expectedWireLen)
	}

	paddingStart := headerLen + plainLen
	for i, b := range wire[paddingStart:] {
		if b != 0 {
			return nil, Stats{}, fmt.Errorf("%w: non-zero padding at offset %d", ErrMalformedFrame, paddingStart+i)
		}
	}

	payload := make([]byte, plainLen)
	copy(payload, wire[headerLen:paddingStart])
	return payload, stats(plainLen, len(wire), paddingLen, cfg.BucketBytes), nil
}

func roundUp(n int, bucket int) (int, error) {
	remainder := n % bucket
	if remainder == 0 {
		return n, nil
	}
	add := bucket - remainder
	if n > math.MaxInt-add {
		return 0, fmt.Errorf("%w: frame length overflows int", ErrInvalidConfig)
	}
	return n + add, nil
}

func stats(plainLen int, wireLen int, paddingLen int, bucketBytes int) Stats {
	return Stats{
		PlainBytes:   plainLen,
		WireBytes:    wireLen,
		PaddingBytes: paddingLen,
		BucketBytes:  bucketBytes,
	}
}
