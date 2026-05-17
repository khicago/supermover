package batching

import (
	"errors"
	"fmt"
	"math"
)

var (
	ErrInvalidConfig = errors.New("invalid batching config")
	ErrRecordTooBig  = errors.New("record too big")
)

type Config struct {
	MaxBytes       int
	MaxCount       int
	FixedBytes     int
	PerRecordBytes int
}

type Batch struct {
	Start int
	Count int
	Bytes int
}

type Stats struct {
	Records  int
	Batches  int
	MaxBytes int
	MaxCount int
}

func Validate(cfg Config) error {
	if cfg.MaxBytes <= 0 {
		return fmt.Errorf("%w: max bytes must be positive", ErrInvalidConfig)
	}
	if cfg.MaxCount <= 0 {
		return fmt.Errorf("%w: max count must be positive", ErrInvalidConfig)
	}
	if cfg.FixedBytes < 0 {
		return fmt.Errorf("%w: fixed bytes must be non-negative", ErrInvalidConfig)
	}
	if cfg.PerRecordBytes < 0 {
		return fmt.Errorf("%w: per-record bytes must be non-negative", ErrInvalidConfig)
	}
	if cfg.FixedBytes > cfg.MaxBytes {
		return fmt.Errorf("%w: fixed bytes exceed max bytes", ErrInvalidConfig)
	}
	return nil
}

func Group(recordSizes []int, cfg Config) ([]Batch, Stats, error) {
	if err := Validate(cfg); err != nil {
		return nil, Stats{}, err
	}
	if len(recordSizes) == 0 {
		return nil, Stats{}, nil
	}

	batches := make([]Batch, 0, len(recordSizes))
	current := Batch{}
	stats := Stats{Records: len(recordSizes)}
	for i, size := range recordSizes {
		if size < 0 {
			return nil, Stats{}, fmt.Errorf("%w: record %d has negative size %d", ErrInvalidConfig, i, size)
		}
		if size > math.MaxInt-cfg.PerRecordBytes {
			return nil, Stats{}, fmt.Errorf("%w: record %d size overflows per-record overhead", ErrInvalidConfig, i)
		}
		recordBytes := size + cfg.PerRecordBytes
		if recordBytes > cfg.MaxBytes-cfg.FixedBytes {
			return nil, Stats{}, fmt.Errorf("%w: record %d size %d exceeds max bytes %d", ErrRecordTooBig, i, size, cfg.MaxBytes)
		}

		if current.Count > 0 && (current.Count == cfg.MaxCount || current.Bytes > math.MaxInt-recordBytes || current.Bytes+recordBytes > cfg.MaxBytes) {
			batches = append(batches, current)
			current = Batch{}
		}
		if current.Count == 0 {
			current.Start = i
			current.Bytes = cfg.FixedBytes
		}
		current.Count++
		current.Bytes += recordBytes
	}
	if current.Count > 0 {
		batches = append(batches, current)
	}

	stats.Batches = len(batches)
	for _, batch := range batches {
		if batch.Bytes > stats.MaxBytes {
			stats.MaxBytes = batch.Bytes
		}
		if batch.Count > stats.MaxCount {
			stats.MaxCount = batch.Count
		}
	}
	return batches, stats, nil
}
