package watcher

import (
	"context"
	"log/slog"
)

type Watcher interface {
	Watch(ctx context.Context, path string) error
	Stop() error
	OnChange(callback func(path string, event EventType))
}

type EventType int

const (
	EventCreate EventType = iota
	EventModify
	EventDelete
)

type StubWatcher struct {
	logger   *slog.Logger
	callback func(path string, event EventType)
}

func NewStubWatcher(logger *slog.Logger) *StubWatcher {
	return &StubWatcher{logger: logger}
}

func (w *StubWatcher) Watch(ctx context.Context, path string) error {
	w.logger.Info("watcher stub: watch requested (v0 does not implement real watching)", "path", path)
	return nil
}

func (w *StubWatcher) Stop() error {
	w.logger.Info("watcher stub: stop requested")
	return nil
}

func (w *StubWatcher) OnChange(callback func(path string, event EventType)) {
	w.callback = callback
}
