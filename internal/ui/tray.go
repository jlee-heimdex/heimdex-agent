package ui

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/getlantern/systray"
	"github.com/heimdex/heimdex-agent/internal/catalog"
)

type Tray struct {
	catalogSvc catalog.CatalogService
	runner     *catalog.Runner
	logger     *slog.Logger

	statusItem  *systray.MenuItem
	sourcesItem *systray.MenuItem
	pauseItem   *systray.MenuItem

	mu sync.Mutex

	onAddFolder func() error
	onQuit      func()
}

type TrayConfig struct {
	CatalogService catalog.CatalogService
	Runner         *catalog.Runner
	Logger         *slog.Logger
	OnAddFolder    func() error
	OnQuit         func()
}

func NewTray(cfg TrayConfig) *Tray {
	return &Tray{
		catalogSvc:  cfg.CatalogService,
		runner:      cfg.Runner,
		logger:      cfg.Logger,
		onAddFolder: cfg.OnAddFolder,
		onQuit:      cfg.OnQuit,
	}
}

func (t *Tray) Run() {
	systray.Run(t.onReady, t.onExit)
}

func (t *Tray) onReady() {
	systray.SetIcon(iconBytes)
	systray.SetTitle("Heimdex")
	systray.SetTooltip("Heimdex Agent")

	t.statusItem = systray.AddMenuItem("Status: Idle", "Current agent status")
	t.statusItem.Disable()

	t.sourcesItem = systray.AddMenuItem("Sources: 0", "Connected sources")
	t.sourcesItem.Disable()

	systray.AddSeparator()

	t.pauseItem = systray.AddMenuItem("Pause", "Pause indexing")

	addFolderItem := systray.AddMenuItem("Add Folder...", "Add a folder to index")

	systray.AddSeparator()

	quitItem := systray.AddMenuItem("Quit", "Quit Heimdex Agent")

	go func() {
		for {
			select {
			case <-t.pauseItem.ClickedCh:
				t.togglePause()
			case <-addFolderItem.ClickedCh:
				t.handleAddFolder()
			case <-quitItem.ClickedCh:
				t.logger.Info("quit requested from tray")
				if t.onQuit != nil {
					t.onQuit()
				}
				systray.Quit()
				return
			}
		}
	}()

	t.logger.Info("system tray ready")
}

func (t *Tray) onExit() {
	t.logger.Info("system tray exiting")
}

func (t *Tray) togglePause() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.runner == nil {
		return
	}

	if t.runner.IsPaused() {
		t.runner.Resume()
		t.pauseItem.SetTitle("Pause")
		t.statusItem.SetTitle("Status: Idle")
	} else {
		t.runner.Pause()
		t.pauseItem.SetTitle("Resume")
		t.statusItem.SetTitle("Status: Paused")
	}
}

func (t *Tray) handleAddFolder() {
	if t.onAddFolder != nil {
		if err := t.onAddFolder(); err != nil {
			t.logger.Error("failed to add folder", "error", err)
		}
	}
}

func (t *Tray) UpdateStatus(status string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.runner != nil && t.runner.IsPaused() {
		return
	}
	t.statusItem.SetTitle("Status: " + status)
}

func (t *Tray) UpdateSourcesCount(count int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sourcesItem.SetTitle(fmt.Sprintf("Sources: %d", count))
}

func (t *Tray) Quit() {
	systray.Quit()
}
