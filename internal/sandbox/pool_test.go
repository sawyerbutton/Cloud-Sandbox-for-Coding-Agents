package sandbox

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPool_AcquireRelease(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultConfig()
	config.Image = "alpine:latest"

	runtime, err := NewDockerRuntime(config)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	poolConfig := PoolConfig{
		MinSize:         1,
		MaxSize:         5,
		WarmupSize:      2,
		IdleTimeout:     5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		SandboxConfig:   config,
	}

	pool := NewPool(poolConfig, runtime)
	defer pool.Close()

	ctx := context.Background()

	// Wait for warmup
	time.Sleep(3 * time.Second)

	// Acquire sandbox
	sb, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire sandbox: %v", err)
	}

	if sb.Status != StatusActive {
		t.Errorf("Expected status %s, got %s", StatusActive, sb.Status)
	}

	stats := pool.Stats()
	if stats["active"] != 1 {
		t.Errorf("Expected 1 active sandbox, got %d", stats["active"])
	}

	// Release sandbox
	if err := pool.Release(ctx, sb.ID); err != nil {
		t.Fatalf("Failed to release sandbox: %v", err)
	}

	stats = pool.Stats()
	if stats["active"] != 0 {
		t.Errorf("Expected 0 active sandbox, got %d", stats["active"])
	}
}

func TestPool_ConcurrentAcquire(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultConfig()
	config.Image = "alpine:latest"

	runtime, err := NewDockerRuntime(config)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	poolConfig := PoolConfig{
		MinSize:         0,
		MaxSize:         10,
		WarmupSize:      0,
		IdleTimeout:     5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		SandboxConfig:   config,
	}

	pool := NewPool(poolConfig, runtime)
	defer pool.Close()

	ctx := context.Background()
	numWorkers := 5

	var wg sync.WaitGroup
	sandboxes := make([]*Sandbox, numWorkers)
	errors := make([]error, numWorkers)

	// Acquire sandboxes concurrently
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sb, err := pool.Acquire(ctx)
			sandboxes[idx] = sb
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Check results
	acquired := 0
	for i := 0; i < numWorkers; i++ {
		if errors[i] != nil {
			t.Errorf("Worker %d failed: %v", i, errors[i])
		} else {
			acquired++
		}
	}

	t.Logf("Successfully acquired %d sandboxes", acquired)

	stats := pool.Stats()
	if stats["active"] != acquired {
		t.Errorf("Expected %d active sandboxes, got %d", acquired, stats["active"])
	}

	// Release all
	for i := 0; i < numWorkers; i++ {
		if sandboxes[i] != nil {
			pool.Release(ctx, sandboxes[i].ID)
		}
	}
}

func TestPool_MaxSize(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultConfig()
	config.Image = "alpine:latest"

	runtime, err := NewDockerRuntime(config)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	poolConfig := PoolConfig{
		MinSize:         0,
		MaxSize:         2,
		WarmupSize:      0,
		IdleTimeout:     5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		SandboxConfig:   config,
	}

	pool := NewPool(poolConfig, runtime)
	defer pool.Close()

	ctx := context.Background()

	// Acquire up to max
	sb1, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire sandbox 1: %v", err)
	}
	defer pool.Release(ctx, sb1.ID)

	sb2, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire sandbox 2: %v", err)
	}
	defer pool.Release(ctx, sb2.ID)

	// Third should fail
	_, err = pool.Acquire(ctx)
	if err != ErrPoolExhausted {
		t.Errorf("Expected ErrPoolExhausted, got %v", err)
	}
}

func TestPool_Stats(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultConfig()
	config.Image = "alpine:latest"

	runtime, err := NewDockerRuntime(config)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	poolConfig := PoolConfig{
		MinSize:         1,
		MaxSize:         10,
		WarmupSize:      3,
		IdleTimeout:     5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		SandboxConfig:   config,
	}

	pool := NewPool(poolConfig, runtime)
	defer pool.Close()

	// Wait for warmup
	time.Sleep(5 * time.Second)

	stats := pool.Stats()

	if stats["max"] != 10 {
		t.Errorf("Expected max 10, got %d", stats["max"])
	}

	if stats["idle"] < 1 {
		t.Errorf("Expected at least 1 idle sandbox after warmup, got %d", stats["idle"])
	}
}
