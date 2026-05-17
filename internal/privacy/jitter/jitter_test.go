package jitter

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "positive budget", cfg: Config{BudgetMillis: 10}},
		{name: "zero disabled", cfg: Config{BudgetMillis: 0}},
		{name: "negative budget", cfg: Config{BudgetMillis: -1}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if tt.wantErr && !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestValidateRejectsBudgetBeyondMaxDurationOn64Bit(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("int cannot represent a budget beyond max duration on this architecture")
	}
	tooLarge := int64(maxBudgetMillis) + 1

	err := Validate(Config{BudgetMillis: int(tooLarge)})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
	}
}

func TestWaitZeroDisabled(t *testing.T) {
	source := &sequenceSource{values: []int{1}}
	sleeper := &recordingSleeper{}
	s, err := NewScheduler(Config{BudgetMillis: 0}, WithSource(source), WithSleeper(sleeper))
	if err != nil {
		t.Fatalf("NewScheduler() error = %v", err)
	}

	stats, err := s.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	wantStats := Stats{BudgetMillis: 0}
	if stats != wantStats {
		t.Fatalf("Wait() stats = %+v, want %+v", stats, wantStats)
	}
	if source.calls != 0 {
		t.Fatalf("source calls = %d, want 0", source.calls)
	}
	if len(sleeper.delays) != 0 {
		t.Fatalf("sleeper delays = %v, want none", sleeper.delays)
	}
}

func TestWaitDeterministicSourceBounds(t *testing.T) {
	source := &sequenceSource{values: []int{0, 5, 10}}
	sleeper := &recordingSleeper{}
	s, err := NewScheduler(Config{BudgetMillis: 10}, WithSource(source), WithSleeper(sleeper))
	if err != nil {
		t.Fatalf("NewScheduler() error = %v", err)
	}

	var got Stats
	for i := 0; i < 3; i++ {
		got, err = s.Wait(context.Background())
		if err != nil {
			t.Fatalf("Wait(%d) error = %v", i, err)
		}
	}

	wantStats := Stats{
		JitteredRequests: 3,
		TotalDelayMillis: 15,
		MaxDelayMillis:   10,
		BudgetMillis:     10,
	}
	if got != wantStats {
		t.Fatalf("Wait() stats = %+v, want %+v", got, wantStats)
	}
	wantDelays := []time.Duration{0, 5 * time.Millisecond, 10 * time.Millisecond}
	if !reflect.DeepEqual(sleeper.delays, wantDelays) {
		t.Fatalf("sleeper delays = %v, want %v", sleeper.delays, wantDelays)
	}
}

func TestWaitRejectsSourceOutOfRange(t *testing.T) {
	tests := []struct {
		name  string
		value int
	}{
		{name: "negative", value: -1},
		{name: "above budget", value: 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &sequenceSource{values: []int{tt.value}}
			sleeper := &recordingSleeper{}
			s, err := NewScheduler(Config{BudgetMillis: 10}, WithSource(source), WithSleeper(sleeper))
			if err != nil {
				t.Fatalf("NewScheduler() error = %v", err)
			}

			stats, err := s.Wait(context.Background())
			if !errors.Is(err, ErrInvalidDelay) {
				t.Fatalf("Wait() error = %v, want ErrInvalidDelay", err)
			}
			wantStats := Stats{BudgetMillis: 10}
			if stats != wantStats {
				t.Fatalf("Wait() stats = %+v, want %+v", stats, wantStats)
			}
			if len(sleeper.delays) != 0 {
				t.Fatalf("sleeper delays = %v, want none", sleeper.delays)
			}
		})
	}
}

func TestWaitContextCanceledBeforeWait(t *testing.T) {
	source := &sequenceSource{values: []int{5}}
	sleeper := &recordingSleeper{}
	s, err := NewScheduler(Config{BudgetMillis: 10}, WithSource(source), WithSleeper(sleeper))
	if err != nil {
		t.Fatalf("NewScheduler() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stats, err := s.Wait(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait() error = %v, want context.Canceled", err)
	}
	wantStats := Stats{BudgetMillis: 10}
	if stats != wantStats {
		t.Fatalf("Wait() stats = %+v, want %+v", stats, wantStats)
	}
	if source.calls != 0 {
		t.Fatalf("source calls = %d, want 0", source.calls)
	}
	if len(sleeper.delays) != 0 {
		t.Fatalf("sleeper delays = %v, want none", sleeper.delays)
	}
}

func TestWaitCanceledDuringSleep(t *testing.T) {
	source := &sequenceSource{values: []int{9}}
	sleeper := newBlockingSleeper()
	s, err := NewScheduler(Config{BudgetMillis: 10}, WithSource(source), WithSleeper(sleeper))
	if err != nil {
		t.Fatalf("NewScheduler() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan waitResult, 1)
	go func() {
		stats, err := s.Wait(ctx)
		done <- waitResult{stats: stats, err: err}
	}()

	gotDelay := sleeper.waitStarted(t)
	if gotDelay != 9*time.Millisecond {
		t.Fatalf("sleep delay = %v, want 9ms", gotDelay)
	}
	cancel()

	result := receiveWaitResult(t, done)
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("Wait() error = %v, want context.Canceled", result.err)
	}
	wantStats := Stats{BudgetMillis: 10}
	if result.stats != wantStats {
		t.Fatalf("Wait() stats = %+v, want %+v", result.stats, wantStats)
	}
}

func TestWaitDeadlineDuringSleep(t *testing.T) {
	source := &sequenceSource{values: []int{7}}
	sleeper := newBlockingSleeper()
	s, err := NewScheduler(Config{BudgetMillis: 10}, WithSource(source), WithSleeper(sleeper))
	if err != nil {
		t.Fatalf("NewScheduler() error = %v", err)
	}
	ctx := newManualDeadlineContext()

	done := make(chan waitResult, 1)
	go func() {
		stats, err := s.Wait(ctx)
		done <- waitResult{stats: stats, err: err}
	}()

	gotDelay := sleeper.waitStarted(t)
	if gotDelay != 7*time.Millisecond {
		t.Fatalf("sleep delay = %v, want 7ms", gotDelay)
	}
	ctx.expire()

	result := receiveWaitResult(t, done)
	if !errors.Is(result.err, context.DeadlineExceeded) {
		t.Fatalf("Wait() error = %v, want context.DeadlineExceeded", result.err)
	}
	wantStats := Stats{BudgetMillis: 10}
	if result.stats != wantStats {
		t.Fatalf("Wait() stats = %+v, want %+v", result.stats, wantStats)
	}
}

type sequenceSource struct {
	values []int
	calls  int
}

func (s *sequenceSource) DelayMillis(int) (int, error) {
	if s.calls >= len(s.values) {
		return 0, errors.New("sequence exhausted")
	}
	value := s.values[s.calls]
	s.calls++
	return value, nil
}

type recordingSleeper struct {
	delays []time.Duration
}

func (s *recordingSleeper) Sleep(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.delays = append(s.delays, delay)
	return nil
}

type blockingSleeper struct {
	started chan time.Duration
}

func newBlockingSleeper() *blockingSleeper {
	return &blockingSleeper{started: make(chan time.Duration, 1)}
}

func (s *blockingSleeper) Sleep(ctx context.Context, delay time.Duration) error {
	s.started <- delay
	<-ctx.Done()
	return ctx.Err()
}

func (s *blockingSleeper) waitStarted(t *testing.T) time.Duration {
	t.Helper()

	select {
	case delay := <-s.started:
		return delay
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for fake sleep to start")
		return 0
	}
}

type manualDeadlineContext struct {
	done chan struct{}
}

func newManualDeadlineContext() *manualDeadlineContext {
	return &manualDeadlineContext{done: make(chan struct{})}
}

func (c *manualDeadlineContext) Deadline() (time.Time, bool) {
	return time.Time{}, true
}

func (c *manualDeadlineContext) Done() <-chan struct{} {
	return c.done
}

func (c *manualDeadlineContext) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func (c *manualDeadlineContext) Value(any) any {
	return nil
}

func (c *manualDeadlineContext) expire() {
	close(c.done)
}

type waitResult struct {
	stats Stats
	err   error
}

func receiveWaitResult(t *testing.T, done <-chan waitResult) waitResult {
	t.Helper()

	select {
	case result := <-done:
		return result
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Wait result")
		return waitResult{}
	}
}
