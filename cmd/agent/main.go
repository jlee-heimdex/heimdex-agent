package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/heimdex/heimdex-agent/internal/api"
	"github.com/heimdex/heimdex-agent/internal/catalog"
	"github.com/heimdex/heimdex-agent/internal/cloud"
	"github.com/heimdex/heimdex-agent/internal/config"
	"github.com/heimdex/heimdex-agent/internal/db"
	"github.com/heimdex/heimdex-agent/internal/logging"
	"github.com/heimdex/heimdex-agent/internal/pipeline"
	"github.com/heimdex/heimdex-agent/internal/pipelines"
	"github.com/heimdex/heimdex-agent/internal/playback"
	"github.com/heimdex/heimdex-agent/internal/ui"
)

var Version = "0.1.0"

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal error: %v", err)
	}
}

func run() error {
	startTime := time.Now()

	cfg, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := os.MkdirAll(cfg.DataDir(), 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}
	if err := os.MkdirAll(cfg.CacheDir(), 0755); err != nil {
		return fmt.Errorf("failed to create cache dir: %w", err)
	}

	logger := logging.NewLogger(cfg.LogLevel())
	logger.Info("starting heimdex agent", "version", Version, "data_dir", cfg.DataDir())

	database, err := db.New(cfg.DBPath(), logger)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer database.Close()

	repo := catalog.NewRepository(database.Conn())

	deviceID, err := ensureDeviceID(repo)
	if err != nil {
		return fmt.Errorf("failed to ensure device ID: %w", err)
	}

	authToken, err := ensureAuthToken(repo)
	if err != nil {
		return fmt.Errorf("failed to ensure auth token: %w", err)
	}

	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║                    HEIMDEX AGENT v0.1.0                   ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════╣")
	fmt.Printf("║  API URL:    http://127.0.0.1:%-27d ║\n", cfg.Port())
	fmt.Printf("║  Auth Token: %-45s ║\n", authToken)
	fmt.Printf("║  Device ID:  %-45s ║\n", deviceID[:16]+"...")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	catalogSvc := catalog.NewService(repo, logger)
	playbackSvc := playback.NewServer(logger)

	var cloudClient cloud.Client
	if cfg.CloudEnabled() && cfg.CloudBaseURL() != "" && cfg.CloudToken() != "" {
		cloudClient = cloud.NewHTTPClient(cfg.CloudBaseURL(), cfg.CloudToken(), cfg.CloudOrgSlug(), logger)
		if httpClient, ok := cloudClient.(*cloud.HTTPClient); ok {
			httpClient.SetDeviceID(deviceID)
		}
		logger.Info("cloud sync enabled", "base_url", cfg.CloudBaseURL(), "org_slug", cfg.CloudOrgSlug())
	} else {
		cloudClient = cloud.NewStubClient(logger)
	}

	cloudClient.RegisterDevice(deviceID)

	pipeCfg := pipelines.Config{
		PythonPath:    cfg.PipelinesPython(),
		ModuleName:    cfg.PipelinesModule(),
		ArtifactsBase: filepath.Join(cfg.DataDir(), "artifacts"),
		DoctorTimeout: cfg.PipelinesTimeoutDoctor(),
		SpeechTimeout: cfg.PipelinesTimeoutSpeech(),
		FacesTimeout:  cfg.PipelinesTimeoutFaces(),
		ScenesTimeout: cfg.PipelinesTimeoutScenes(),
		Logger:        logger,
	}

	var pipeRunner pipelines.Runner
	var doctor *pipelines.CachedDoctor

	pr, err := pipelines.NewRunner(pipeCfg)
	if err != nil {
		logger.Warn("pipeline runner unavailable, indexing disabled", "error", err)
	} else {
		pipeRunner = pr
		doctor = pipelines.NewCachedDoctor(pr, logger)

		initCtx, initCancel := context.WithTimeout(context.Background(), pipeCfg.DoctorTimeout)
		defer initCancel()
		if caps, err := doctor.Refresh(initCtx); err != nil {
			logger.Warn("initial doctor probe failed", "error", err)
		} else {
			logger.Info("pipeline capabilities detected",
				"faces", caps.HasFaces,
				"speech", caps.HasSpeech,
				"deps", fmt.Sprintf("%d/%d", caps.Summary.Available, caps.Summary.Total),
			)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ffmpeg := pipeline.NewRealFFmpeg(logger)

	runner := catalog.NewRunner(catalogSvc, repo, pipeRunner, ffmpeg, doctor, logger)
	runner.SetOCRConfig(cfg)
	if cfg.CloudEnabled() {
		runner.SetCloudClient(cloudClient, cfg.CloudLibraryID())
	}
	go runner.Start(ctx)

	apiServer := api.NewServer(api.ServerConfig{
		Port:           cfg.Port(),
		ArtifactsDir:   pipeCfg.ArtifactsBase,
		CatalogService: catalogSvc,
		PlaybackServer: playbackSvc,
		Repository:     repo,
		Runner:         runner,
		Doctor:         doctor,
		Logger:         logger,
		StartTime:      startTime,
		DeviceID:       deviceID,
	})

	go func() {
		if err := apiServer.Start(); err != nil {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	quitCh := make(chan struct{})

	go func() {
		select {
		case sig := <-sigCh:
			logger.Info("received shutdown signal", "signal", sig)
			close(quitCh)
		case <-quitCh:
		}
	}()

	if cfg.Headless() {
		logger.Info("running in headless mode (no system tray)")
	} else {
		tray := ui.NewTray(ui.TrayConfig{
			CatalogService: catalogSvc,
			Runner:         runner,
			Logger:         logger,
			OnAddFolder: func() error {
				logger.Info("add folder requested from tray (file dialog not implemented in v0)")
				return nil
			},
			OnQuit: func() {
				close(quitCh)
			},
		})
		go tray.Run()
	}

	<-quitCh

	logger.Info("initiating graceful shutdown")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown HTTP server", "error", err)
	}

	logger.Info("shutdown complete")
	return nil
}

func ensureDeviceID(repo catalog.Repository) (string, error) {
	ctx := context.Background()

	existing, err := repo.GetConfig(ctx, "device_id")
	if err == nil && existing != "" {
		return existing, nil
	}

	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", err
	}
	deviceID := hex.EncodeToString(idBytes)

	if err := repo.SetConfig(ctx, "device_id", deviceID); err != nil {
		return "", err
	}

	return deviceID, nil
}

func ensureAuthToken(repo catalog.Repository) (string, error) {
	ctx := context.Background()

	existing, err := repo.GetConfig(ctx, "auth_token")
	if err == nil && existing != "" {
		return existing, nil
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes)

	if err := repo.SetConfig(ctx, "auth_token", token); err != nil {
		return "", err
	}

	return token, nil
}
