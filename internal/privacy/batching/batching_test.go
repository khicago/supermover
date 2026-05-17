package batching

import (
	"errors"
	"reflect"
	"testing"
)

func TestGroupByteBound(t *testing.T) {
	cfg := Config{MaxBytes: 10, MaxCount: 10}

	batches, stats, err := Group([]int{4, 4, 4}, cfg)
	if err != nil {
		t.Fatalf("Group() error = %v", err)
	}

	wantBatches := []Batch{
		{Start: 0, Count: 2, Bytes: 8},
		{Start: 2, Count: 1, Bytes: 4},
	}
	if !reflect.DeepEqual(batches, wantBatches) {
		t.Fatalf("Group() batches = %+v, want %+v", batches, wantBatches)
	}
	wantStats := Stats{Records: 3, Batches: 2, MaxBytes: 8, MaxCount: 2}
	if stats != wantStats {
		t.Fatalf("Group() stats = %+v, want %+v", stats, wantStats)
	}
}

func TestGroupCountBound(t *testing.T) {
	cfg := Config{MaxBytes: 100, MaxCount: 2}

	batches, stats, err := Group([]int{1, 1, 1}, cfg)
	if err != nil {
		t.Fatalf("Group() error = %v", err)
	}

	wantBatches := []Batch{
		{Start: 0, Count: 2, Bytes: 2},
		{Start: 2, Count: 1, Bytes: 1},
	}
	if !reflect.DeepEqual(batches, wantBatches) {
		t.Fatalf("Group() batches = %+v, want %+v", batches, wantBatches)
	}
	wantStats := Stats{Records: 3, Batches: 2, MaxBytes: 2, MaxCount: 2}
	if stats != wantStats {
		t.Fatalf("Group() stats = %+v, want %+v", stats, wantStats)
	}
}

func TestGroupExactBoundary(t *testing.T) {
	cfg := Config{MaxBytes: 10, MaxCount: 3}

	batches, stats, err := Group([]int{3, 7, 1}, cfg)
	if err != nil {
		t.Fatalf("Group() error = %v", err)
	}

	wantBatches := []Batch{
		{Start: 0, Count: 2, Bytes: 10},
		{Start: 2, Count: 1, Bytes: 1},
	}
	if !reflect.DeepEqual(batches, wantBatches) {
		t.Fatalf("Group() batches = %+v, want %+v", batches, wantBatches)
	}
	wantStats := Stats{Records: 3, Batches: 2, MaxBytes: 10, MaxCount: 2}
	if stats != wantStats {
		t.Fatalf("Group() stats = %+v, want %+v", stats, wantStats)
	}
}

func TestGroupRejectsOversizedRecord(t *testing.T) {
	cfg := Config{MaxBytes: 10, MaxCount: 3}

	batches, stats, err := Group([]int{1, 11}, cfg)
	if !errors.Is(err, ErrRecordTooBig) {
		t.Fatalf("Group() error = %v, want ErrRecordTooBig", err)
	}
	if batches != nil {
		t.Fatalf("Group() batches = %+v, want nil", batches)
	}
	if stats != (Stats{}) {
		t.Fatalf("Group() stats = %+v, want zero value", stats)
	}
}

func TestGroupAccountsEnvelopeOverhead(t *testing.T) {
	cfg := Config{MaxBytes: 10, MaxCount: 10, FixedBytes: 2, PerRecordBytes: 1}

	batches, stats, err := Group([]int{3, 3, 1}, cfg)
	if err != nil {
		t.Fatalf("Group() error = %v", err)
	}

	wantBatches := []Batch{
		{Start: 0, Count: 2, Bytes: 10},
		{Start: 2, Count: 1, Bytes: 4},
	}
	if !reflect.DeepEqual(batches, wantBatches) {
		t.Fatalf("Group() batches = %+v, want %+v", batches, wantBatches)
	}
	wantStats := Stats{Records: 3, Batches: 2, MaxBytes: 10, MaxCount: 2}
	if stats != wantStats {
		t.Fatalf("Group() stats = %+v, want %+v", stats, wantStats)
	}
}

func TestGroupEmptyInput(t *testing.T) {
	cfg := Config{MaxBytes: 10, MaxCount: 3}

	batches, stats, err := Group(nil, cfg)
	if err != nil {
		t.Fatalf("Group() error = %v", err)
	}
	if batches != nil {
		t.Fatalf("Group() batches = %+v, want nil", batches)
	}
	wantStats := Stats{}
	if stats != wantStats {
		t.Fatalf("Group() stats = %+v, want %+v", stats, wantStats)
	}
}

func TestValidateRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "zero max bytes", cfg: Config{MaxBytes: 0, MaxCount: 1}},
		{name: "negative max bytes", cfg: Config{MaxBytes: -1, MaxCount: 1}},
		{name: "zero max count", cfg: Config{MaxBytes: 1, MaxCount: 0}},
		{name: "negative max count", cfg: Config{MaxBytes: 1, MaxCount: -1}},
		{name: "negative fixed bytes", cfg: Config{MaxBytes: 1, MaxCount: 1, FixedBytes: -1}},
		{name: "negative per record bytes", cfg: Config{MaxBytes: 1, MaxCount: 1, PerRecordBytes: -1}},
		{name: "fixed bytes exceed max bytes", cfg: Config{MaxBytes: 1, MaxCount: 1, FixedBytes: 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.cfg); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestGroupRejectsNegativeRecordSize(t *testing.T) {
	cfg := Config{MaxBytes: 10, MaxCount: 3}

	_, _, err := Group([]int{1, -1}, cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Group() error = %v, want ErrInvalidConfig", err)
	}
}
