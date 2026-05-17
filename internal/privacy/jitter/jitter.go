package jitter

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
)

const maxBudgetMillis = int64(1<<63-1) / int64(time.Millisecond)

var (
	ErrInvalidConfig = errors.New("invalid jitter config")
	ErrInvalidDelay  = errors.New("invalid jitter delay")
)

type Config struct {
	BudgetMillis int
}

type Stats struct {
	JitteredRequests int
	TotalDelayMillis int
	MaxDelayMillis   int
	BudgetMillis     int
}

type Source interface {
	DelayMillis(budgetMillis int) (int, error)
}

type Sleeper interface {
	Sleep(ctx context.Context, delay time.Duration) error
}

type Option func(*Scheduler) error

type Scheduler struct {
	cfg     Config
	source  Source
	sleeper Sleeper

	mu    sync.Mutex
	stats Stats
}

type CryptoSource struct{}

type ContextSleeper struct{}

func Validate(cfg Config) error {
	if cfg.BudgetMillis < 0 {
		return fmt.Errorf("%w: budget millis must be non-negative", ErrInvalidConfig)
	}
	if int64(cfg.BudgetMillis) > maxBudgetMillis {
		return fmt.Errorf("%w: budget millis exceeds max sleep duration", ErrInvalidConfig)
	}
	return nil
}

func NewScheduler(cfg Config, opts ...Option) (*Scheduler, error) {
	if err := Validate(cfg); err != nil {
		return nil, err
	}

	s := &Scheduler{
		cfg:     cfg,
		source:  CryptoSource{},
		sleeper: ContextSleeper{},
		stats: Stats{
			BudgetMillis: cfg.BudgetMillis,
		},
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func WithSource(source Source) Option {
	return func(s *Scheduler) error {
		if source == nil {
			return fmt.Errorf("%w: source is nil", ErrInvalidConfig)
		}
		s.source = source
		return nil
	}
}

func WithSleeper(sleeper Sleeper) Option {
	return func(s *Scheduler) error {
		if sleeper == nil {
			return fmt.Errorf("%w: sleeper is nil", ErrInvalidConfig)
		}
		s.sleeper = sleeper
		return nil
	}
}

func (s *Scheduler) Wait(ctx context.Context) (Stats, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil {
		return Stats{}, errors.New("nil jitter scheduler")
	}
	if s.cfg.BudgetMillis == 0 {
		return s.Stats(), nil
	}
	if err := ctx.Err(); err != nil {
		return s.Stats(), err
	}

	source := s.source
	if source == nil {
		source = CryptoSource{}
	}
	delayMillis, err := source.DelayMillis(s.cfg.BudgetMillis)
	if err != nil {
		return s.Stats(), fmt.Errorf("sample jitter delay: %w", err)
	}
	if delayMillis < 0 || delayMillis > s.cfg.BudgetMillis {
		return s.Stats(), fmt.Errorf("%w: source returned %d outside [0,%d]", ErrInvalidDelay, delayMillis, s.cfg.BudgetMillis)
	}

	sleeper := s.sleeper
	if sleeper == nil {
		sleeper = ContextSleeper{}
	}
	if err := sleeper.Sleep(ctx, time.Duration(delayMillis)*time.Millisecond); err != nil {
		return s.Stats(), err
	}
	return s.record(delayMillis), nil
}

func (s *Scheduler) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

func (CryptoSource) DelayMillis(budgetMillis int) (int, error) {
	if budgetMillis < 0 {
		return 0, fmt.Errorf("%w: budget millis must be non-negative", ErrInvalidConfig)
	}
	max := big.NewInt(int64(budgetMillis))
	max.Add(max, big.NewInt(1))
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func (ContextSleeper) Sleep(ctx context.Context, delay time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *Scheduler) record(delayMillis int) Stats {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats.JitteredRequests++
	s.stats.TotalDelayMillis += delayMillis
	if delayMillis > s.stats.MaxDelayMillis {
		s.stats.MaxDelayMillis = delayMillis
	}
	return s.stats
}
