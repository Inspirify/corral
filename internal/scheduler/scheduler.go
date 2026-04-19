// Package scheduler manages cron-based scheduling of agents.
package scheduler

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/Inspirify/corral/internal/config"
	"github.com/Inspirify/corral/internal/harness"
	"github.com/robfig/cron/v3"
)

// Scheduler runs agents on their configured cron schedules.
type Scheduler struct {
	cron    *cron.Cron
	cfg     *config.Config
	mu      sync.Mutex
	running map[string]context.CancelFunc
}

// New creates a scheduler from the given configuration.
func New(cfg *config.Config) *Scheduler {
	return &Scheduler{
		cron:    cron.New(cron.WithSeconds()),
		cfg:     cfg,
		running: make(map[string]context.CancelFunc),
	}
}

// Start registers all scheduled agents and starts the cron engine.
func (s *Scheduler) Start(ctx context.Context) error {
	for name, agent := range s.cfg.Agents {
		schedule := agent.Schedule()
		if schedule == "" {
			continue // manual-only agent
		}

		// Capture for closure
		agentName := name
		agentCfg := agent

		_, err := s.cron.AddFunc(schedule, func() {
			s.runAgent(ctx, agentName, agentCfg)
		})
		if err != nil {
			return fmt.Errorf("scheduling agent %q: %w", agentName, err)
		}
		fmt.Printf("[corral] scheduled agent=%s cron=%q\n", agentName, schedule)
	}

	s.cron.Start()
	fmt.Println("[corral] scheduler started")

	// Block until context cancelled
	<-ctx.Done()
	fmt.Println("[corral] scheduler stopping...")
	stopCtx := s.cron.Stop()
	<-stopCtx.Done()

	// Cancel any running agents
	s.mu.Lock()
	for name, cancel := range s.running {
		fmt.Printf("[corral] cancelling agent=%s\n", name)
		cancel()
	}
	s.mu.Unlock()

	return nil
}

// runAgent executes a single agent, applying jitter if configured.
func (s *Scheduler) runAgent(ctx context.Context, name string, agent config.AgentConfig) {
	// Apply jitter
	if jitter := agent.Jitter.Duration; jitter > 0 {
		delay := time.Duration(rand.Int63n(int64(jitter)))
		fmt.Printf("[corral] agent=%s jitter=%v\n", name, delay.Round(time.Second))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return
		}
	}

	// Check if already running (lock handles this too, but avoid spawning)
	s.mu.Lock()
	if _, ok := s.running[name]; ok {
		s.mu.Unlock()
		fmt.Printf("[corral] agent=%s skipped (already running)\n", name)
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.running[name] = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.running, name)
		s.mu.Unlock()
	}()

	h := harness.New(name, agent)
	if err := h.Run(runCtx); err != nil {
		fmt.Printf("[corral] agent=%s error: %v\n", name, err)
	}
	cancel()
}
