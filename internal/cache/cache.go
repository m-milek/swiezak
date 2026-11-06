package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FetchFunc func(ctx context.Context) ([]byte, error)

type Cache struct {
	path     string
	interval time.Duration
	fetch    FetchFunc

	firstWait time.Duration

	mu   sync.RWMutex
	data []byte
}

func New(path string, interval time.Duration, fetch FetchFunc) *Cache {
	c := &Cache{path: path, interval: interval, fetch: fetch}
	c.firstWait = c.load()
	return c
}

func (c *Cache) Get() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data
}

func (c *Cache) Run(ctx context.Context) {
	wait := c.firstWait
	for {
		if wait > 0 {
			slog.Info("next refresh", "in", wait.Round(time.Second))
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}

		slog.Info("refreshing")
		if err := c.refresh(ctx); err != nil {
			slog.Error("refresh failed", "err", err)
		} else {
			slog.Info("saved", "path", c.path)
		}
		wait = c.interval
	}
}

func (c *Cache) load() time.Duration {
	info, err := os.Stat(c.path)
	if err != nil {
		return 0
	}
	data, err := os.ReadFile(c.path)
	if err != nil || !json.Valid(data) {
		slog.Warn("ignoring unreadable cache file", "path", c.path, "err", err)
		return 0
	}

	c.set(data)
	return c.interval - time.Since(info.ModTime())
}

func (c *Cache) refresh(ctx context.Context) error {
	data, err := c.fetch(ctx)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(c.path, data); err != nil {
		return err
	}
	c.set(data)
	return nil
}

func (c *Cache) set(data []byte) {
	c.mu.Lock()
	c.data = data
	c.mu.Unlock()
}

func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cache-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
