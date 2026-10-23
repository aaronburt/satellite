package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"satellite/internal/capabilities"
	"satellite/internal/config"
	"satellite/internal/logger"
	"satellite/internal/mqtt"
	"satellite/internal/output"
	"satellite/internal/telemetry"
	"satellite/internal/toast"
	"satellite/internal/updater"
	"satellite/internal/webui"
)

func runCLI(collector *telemetry.Collector, cfg config.Config, updaterInstance *updater.Checker) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if cfg.CheckUpdates && updaterInstance != nil {
		updaterInstance.StartBackground(ctx, updater.DefaultCheckInterval)
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nShutting down satellite agent.")
			return
		case <-ticker.C:
			snap := collector.Collect(cfg.Expose)

			var update *updater.CheckStatus
			if updaterInstance != nil {
				st := updaterInstance.Status()
				update = &st
			}
			fmt.Print("\033[H\033[2J")
			renderCLI(os.Stdout, snap, update)
		}
	}
}

func main() {
	debug.SetGCPercent(25)
	debug.SetMemoryLimit(16 * 1024 * 1024)

	cliMode := flag.Bool("cli", false, "Run in terminal CLI/TUI mode")
	debugMode := flag.Bool("debug", false, "Run in debug mode (enables WebUI automatically with hot reload from disk)")
	headlessMode := flag.Bool("headless", false, "Run in headless background mode without system tray")
	exportPath := flag.String("export", "", "Export configuration to specified file path and exit")
	importPath := flag.String("import", "", "Import configuration from specified file path and exit")
	passwordFlag := flag.String("password", "", "Passphrase for configuration export encryption or import decryption")
	resetNodeID := flag.Bool("reset-node-id", true, "Adapt Node ID and Client ID to current machine hostname on import")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	if handled, code := runConfigCommand(cfg, *exportPath, *importPath, *passwordFlag, *resetNodeID, os.Stdout, os.Stderr); handled {
		if code != 0 {
			os.Exit(code)
		}
		return
	}

	logger.Init(cfg.Webhook, cfg.NodeID)
	logger.Info("system", fmt.Sprintf("Satellite agent v%s started on %s", config.Version, cfg.NodeID))

	caps := capabilities.Detect()
	collector := telemetry.NewCollector()

	updaterInstance := updater.NewChecker(cfg.UpdateRepo, config.Version, func(status updater.CheckStatus) {
		_ = toast.Show(toast.Notification{
			Title:   "Satellite Update Available",
			Message: fmt.Sprintf("Version %s is available. Click to download.", status.LatestVersion),
			URL:     status.ReleaseURL,
		})
	})
	updaterInstance.SetEnabled(cfg.CheckUpdates)

	if *cliMode {
		runCLI(collector, cfg, updaterInstance)
		return
	}

	initialSnap := collector.Collect(cfg.Expose)
	telemetry.TrimWorkingSet()

	mqttClient := mqtt.NewClient()
	mqttClient.SetCapabilities(caps)
	mqttClient.SetHasBattery(initialSnap.HasBattery)
	mqttClient.Start(cfg)

	server := webui.NewServer(collector, mqttClient)
	server.SetCapabilities(caps)
	server.SetUpdater(updaterInstance)
	server.SetWebUIEnabled(*debugMode)
	server.SetDevMode(*debugMode)
	if *debugMode {
		if _, err := server.Start(cfg.GetPort()); err == nil {
			fmt.Printf("\n[DEBUG] WebUI running with live disk reload at: %s\n\n", server.URL())
		}
	} else if cfg.JSONEnabled {
		if cfg.APIKey == "" {
			cfg.APIKey = config.GenerateAPIKey()
			_ = config.Save(cfg)
		}
		server.SetJSONEnabled(true)
		server.SetAPIKey(cfg.APIKey)
		_, _ = server.Start(cfg.GetPort())
	}

	updaterCtx, cancelUpdater := context.WithCancel(context.Background())
	updaterInstance.StartBackground(updaterCtx, updater.DefaultCheckInterval)

	// Every snapshot goes to each output: MQTT for Home Assistant, plus the
	// optional generic telemetry webhook.
	sinks := []output.Sink{
		output.SinkFunc{SinkName: "mqtt", Fn: mqttClient.PublishTelemetry},
		output.NewWebhook(config.Get),
	}

	tickerCtx, cancelTicker := context.WithCancel(context.Background())
	go func() {
		interval := time.Duration(cfg.IntervalSec) * time.Second
		if interval < 1*time.Second {
			interval = 5 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var tickCount uint64
		for {
			select {
			case <-tickerCtx.Done():
				return
			case <-ticker.C:
				tickCount++
				currentCfg := config.Get()
				snap := collector.Collect(currentCfg.Expose)
				output.PublishAll(sinks, snap)
				if tickCount%12 == 0 {
					telemetry.TrimWorkingSet()
				}
			}
		}
	}()

	onExit := func() {
		logger.Info("system", "Satellite agent shutting down")
		cancelUpdater()
		cancelTicker()
		mqttClient.Stop()
		server.Stop()
	}

	if *debugMode || *headlessMode {
		sigCtx, sigCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer sigCancel()
		<-sigCtx.Done()
		onExit()
		return
	}

	runTray(server, mqttClient, updaterInstance, onExit)
}
