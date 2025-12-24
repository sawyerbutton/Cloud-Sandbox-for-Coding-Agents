package sandbox

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var (
	ErrPoolExhausted = errors.New("sandbox pool exhausted")
	ErrPoolClosed    = errors.New("sandbox pool is closed")
)

// PoolConfig holds pool configuration
type PoolConfig struct {
	// Minimum number of idle sandboxes to maintain
	MinSize int `yaml:"min_size"`

	// Maximum number of total sandboxes
	MaxSize int `yaml:"max_size"`

	// Number of sandboxes to pre-warm
	WarmupSize int `yaml:"warmup_size"`

	// Time after which idle sandboxes are cleaned up
	IdleTimeout time.Duration `yaml:"idle_timeout"`

	// Interval for cleanup checks
	CleanupInterval time.Duration `yaml:"cleanup_interval"`

	// Sandbox configuration
	SandboxConfig Config `yaml:"sandbox_config"`
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MinSize:         2,
		MaxSize:         50,
		WarmupSize:      5,
		IdleTimeout:     30 * time.Minute,
		CleanupInterval: 5 * time.Minute,
		SandboxConfig:   DefaultConfig(),
	}
}

// Pool manages a pool of sandboxes
type Pool struct {
	config  PoolConfig
	runtime Runtime

	mu       sync.RWMutex
	idle     []*Sandbox
	active   map[string]*Sandbox
	creating int

	stopCh chan struct{}
	wg     sync.WaitGroup
	closed bool
}

// NewPool creates a new sandbox pool
func NewPool(config PoolConfig, runtime Runtime) *Pool {
	p := &Pool{
		config:  config,
		runtime: runtime,
		idle:    make([]*Sandbox, 0, config.MaxSize),
		active:  make(map[string]*Sandbox),
		stopCh:  make(chan struct{}),
	}

	// Start background goroutines
	p.wg.Add(2)
	go p.warmupLoop()
	go p.cleanupLoop()

	return p
}

// Acquire acquires a sandbox from the pool
func (p *Pool) Acquire(ctx context.Context) (*Sandbox, error) {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}

	// Try to get from idle pool
	if len(p.idle) > 0 {
		sb := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]
		p.active[sb.ID] = sb
		p.mu.Unlock()

		sb.Status = StatusActive
		sb.LastActiveAt = time.Now()

		log.Printf("[Pool] Acquired sandbox %s from idle pool", sb.ID)
		return sb, nil
	}

	// Check if we can create a new one
	totalCount := len(p.active) + len(p.idle) + p.creating
	if totalCount >= p.config.MaxSize {
		p.mu.Unlock()
		return nil, ErrPoolExhausted
	}

	p.creating++
	p.mu.Unlock()

	// Create new sandbox
	sb, err := p.runtime.Create(ctx, p.config.SandboxConfig)
	if err != nil {
		p.mu.Lock()
		p.creating--
		p.mu.Unlock()
		return nil, err
	}

	p.mu.Lock()
	p.creating--
	p.active[sb.ID] = sb
	p.mu.Unlock()

	sb.Status = StatusActive
	log.Printf("[Pool] Created new sandbox %s", sb.ID)

	return sb, nil
}

// Release releases a sandbox back to the pool
func (p *Pool) Release(ctx context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	sb, ok := p.active[id]
	if !ok {
		return errors.New("sandbox not found in active pool")
	}

	delete(p.active, id)

	// Reset and return to idle pool if there's room
	if len(p.idle) < p.config.MaxSize && !p.closed {
		sb.Status = StatusIdle
		sb.LastActiveAt = time.Now()
		p.idle = append(p.idle, sb)
		log.Printf("[Pool] Released sandbox %s to idle pool", sb.ID)
		return nil
	}

	// Otherwise destroy it
	go func() {
		if err := p.runtime.Destroy(context.Background(), sb.ID); err != nil {
			log.Printf("[Pool] Failed to destroy sandbox %s: %v", sb.ID, err)
		}
		log.Printf("[Pool] Destroyed sandbox %s (pool full)", sb.ID)
	}()

	return nil
}

// Destroy removes a sandbox from the pool and destroys it
func (p *Pool) Destroy(ctx context.Context, id string) error {
	p.mu.Lock()

	// Check active pool
	if sb, ok := p.active[id]; ok {
		delete(p.active, id)
		p.mu.Unlock()
		return p.runtime.Destroy(ctx, sb.ID)
	}

	// Check idle pool
	for i, sb := range p.idle {
		if sb.ID == id {
			p.idle = append(p.idle[:i], p.idle[i+1:]...)
			p.mu.Unlock()
			return p.runtime.Destroy(ctx, sb.ID)
		}
	}

	p.mu.Unlock()
	return errors.New("sandbox not found")
}

// Get returns a sandbox by ID (must be active)
func (p *Pool) Get(ctx context.Context, id string) (*Sandbox, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if sb, ok := p.active[id]; ok {
		return sb, nil
	}

	return nil, errors.New("sandbox not found or not active")
}

// Stats returns pool statistics
func (p *Pool) Stats() map[string]int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]int{
		"idle":     len(p.idle),
		"active":   len(p.active),
		"creating": p.creating,
		"max":      p.config.MaxSize,
	}
}

// warmupLoop maintains the minimum number of idle sandboxes
func (p *Pool) warmupLoop() {
	defer p.wg.Done()

	// Initial warmup
	p.warmup()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.warmup()
		}
	}
}

func (p *Pool) warmup() {
	p.mu.RLock()
	needed := p.config.WarmupSize - len(p.idle) - p.creating
	p.mu.RUnlock()

	if needed <= 0 {
		return
	}

	log.Printf("[Pool] Warming up %d sandboxes", needed)

	var wg sync.WaitGroup
	for i := 0; i < needed; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			p.mu.Lock()
			if len(p.idle)+len(p.active)+p.creating >= p.config.MaxSize {
				p.mu.Unlock()
				return
			}
			p.creating++
			p.mu.Unlock()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			sb, err := p.runtime.Create(ctx, p.config.SandboxConfig)
			if err != nil {
				log.Printf("[Pool] Failed to warm up sandbox: %v", err)
				p.mu.Lock()
				p.creating--
				p.mu.Unlock()
				return
			}

			p.mu.Lock()
			p.creating--
			p.idle = append(p.idle, sb)
			p.mu.Unlock()

			log.Printf("[Pool] Warmed up sandbox %s", sb.ID)
		}()
	}

	wg.Wait()
}

// cleanupLoop periodically cleans up idle sandboxes
func (p *Pool) cleanupLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.cleanup()
		}
	}
}

func (p *Pool) cleanup() {
	p.mu.Lock()

	ctx := context.Background()
	now := time.Now()
	var toRemove []int
	var toDestroy []*Sandbox

	// Find idle sandboxes that have timed out
	for i, sb := range p.idle {
		if now.Sub(sb.LastActiveAt) > p.config.IdleTimeout {
			// Keep minimum pool size
			if len(p.idle)-len(toRemove) <= p.config.MinSize {
				break
			}
			toRemove = append(toRemove, i)
			toDestroy = append(toDestroy, sb)
		}
	}

	// Remove from idle pool (in reverse order to maintain indices)
	for i := len(toRemove) - 1; i >= 0; i-- {
		idx := toRemove[i]
		p.idle = append(p.idle[:idx], p.idle[idx+1:]...)
	}

	// Also check active sandboxes for stuck ones
	for id, sb := range p.active {
		if now.Sub(sb.LastActiveAt) > p.config.IdleTimeout*2 {
			delete(p.active, id)
			toDestroy = append(toDestroy, sb)
			log.Printf("[Pool] Cleaning up stuck active sandbox %s", sb.ID)
		}
	}

	p.mu.Unlock()

	// Destroy sandboxes outside lock
	for _, sb := range toDestroy {
		if err := p.runtime.Destroy(ctx, sb.ID); err != nil {
			log.Printf("[Pool] Failed to cleanup sandbox %s: %v", sb.ID, err)
		} else {
			log.Printf("[Pool] Cleaned up idle sandbox %s", sb.ID)
		}
	}
}

// Close closes the pool and destroys all sandboxes
func (p *Pool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true

	// Collect all sandboxes
	var all []*Sandbox
	all = append(all, p.idle...)
	for _, sb := range p.active {
		all = append(all, sb)
	}
	p.idle = nil
	p.active = make(map[string]*Sandbox)
	p.mu.Unlock()

	// Stop background goroutines
	close(p.stopCh)
	p.wg.Wait()

	// Destroy all sandboxes
	ctx := context.Background()
	for _, sb := range all {
		if err := p.runtime.Destroy(ctx, sb.ID); err != nil {
			log.Printf("[Pool] Failed to destroy sandbox %s on close: %v", sb.ID, err)
		}
	}

	log.Printf("[Pool] Closed pool, destroyed %d sandboxes", len(all))
	return nil
}
