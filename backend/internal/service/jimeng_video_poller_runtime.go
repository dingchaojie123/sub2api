package service

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type JimengVideoPollerRuntime struct {
	poller   *JimengVideoPollerService
	ppPoller *PPVideoPollerService
	cfg      *config.Config

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewJimengVideoPollerRuntime(poller *JimengVideoPollerService, cfg *config.Config) *JimengVideoPollerRuntime {
	return &JimengVideoPollerRuntime{poller: poller, cfg: cfg}
}

func (r *JimengVideoPollerRuntime) SetPPVideoPoller(poller *PPVideoPollerService) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ppPoller = poller
}

func (r *JimengVideoPollerRuntime) Start() {
	if r == nil || r.cfg == nil || !r.cfg.JimengVideo.PollerEnabled {
		return
	}
	if (r.poller == nil || r.poller.Repo == nil || r.poller.Gateway == nil) &&
		(r.ppPoller == nil || r.ppPoller.Repo == nil || r.ppPoller.Gateway == nil) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	r.cancel = cancel
	r.done = done

	go func() {
		defer close(done)
		r.run(ctx)
	}()
}

func (r *JimengVideoPollerRuntime) run(ctx context.Context) {
	if r == nil || (r.poller == nil && r.ppPoller == nil) {
		return
	}
	interval := 30 * time.Second
	if r.poller != nil {
		interval = r.poller.options().PollInterval
	} else if r.ppPoller != nil {
		interval = r.ppPoller.options().PollInterval
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if r.poller != nil && r.poller.Repo != nil && r.poller.Gateway != nil {
			stats, err := r.poller.RunOnce(ctx)
			if err != nil {
				logger.L().Warn("jimeng_video.poller.run_failed",
					zap.Int("claimed", stats.Claimed),
					zap.Int("errors", stats.Errors),
					zap.Error(err),
				)
			}
		}
		if r.ppPoller != nil && r.ppPoller.Repo != nil && r.ppPoller.Gateway != nil {
			stats, err := r.ppPoller.RunOnce(ctx)
			if err != nil {
				logger.L().Warn("pp_video.poller.run_failed",
					zap.Int("claimed", stats.Claimed),
					zap.Int("errors", stats.Errors),
					zap.Error(err),
				)
			}
		}
		sleepOrDone(ctx, interval)
	}
}

func (r *JimengVideoPollerRuntime) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	r.cancel = nil
	r.done = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (r *JimengVideoPollerRuntime) Running() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancel != nil
}
