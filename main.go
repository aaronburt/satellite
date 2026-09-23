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

			fmt.Print("\033[H\033[2J")
			fmt.Printf("========================================\n")
			fmt.Printf("       SATELLITE AGENT (v%s CLI)       \n", config.Version)
			fmt.Printf("========================================\n")
			if updaterInstance != nil {
				if updateStatus := updaterInstance.Status(); updateStatus.Available {
					fmt.Printf(" Update       : v%s available! (%s)\n", updateStatus.LatestVersion, updateStatus.ReleaseURL)
				}
			}
			if snap.CPUPercent != nil {
				fmt.Printf(" Processor    : %6.1f %%\n", *snap.CPUPercent)
			}
			if snap.MemoryUsedGB != nil && snap.MemoryTotalGB != nil && snap.MemoryPercent != nil {
				fmt.Printf(" Memory (RAM) : %6.2f / %6.2f GB (%5.1f %%)\n", *snap.MemoryUsedGB, *snap.MemoryTotalGB, *snap.MemoryPercent)
			}
			for _, g := range snap.GPUs {
				gpuStr := ""
				if g.CoreUsagePercent != nil {
					gpuStr += fmt.Sprintf("%5.1f %%", *g.CoreUsagePercent)
				}
				if g.MemoryUsedMB != nil && g.MemoryTotalMB != nil {
					usedGB := *g.MemoryUsedMB / 1024
					totGB := *g.MemoryTotalMB / 1024
					gpuStr += fmt.Sprintf(" | VRAM: %4.1f / %4.1f GB", usedGB, totGB)
					if g.MemoryPercent != nil {
						gpuStr += fmt.Sprintf(" (%4.1f %%)", *g.MemoryPercent)
					}
				}
				if g.TemperatureC != nil {
					gpuStr += fmt.Sprintf(" | %2.0f °C", *g.TemperatureC)
				}
				name := g.Name
				if name == "" {
					name = fmt.Sprintf("GPU %d", g.Index)
				}
				fmt.Printf(" GPU (%s) : %s\n", name, gpuStr)
			}
			for _, d := range snap.Drives {
				fmt.Printf(" Storage (%s) : %6.1f / %6.1f GB (%5.1f %%)\n", d.Mount, d.UsedGB, d.TotalGB, d.UsedPercent)
			}
			if snap.NetworkRxKBs != nil && snap.NetworkTxKBs != nil {
				fmt.Printf(" Network I/O  : ↓ %7.1f kB/s | ↑ %7.1f kB/s\n", *snap.NetworkRxKBs, *snap.NetworkTxKBs)
			}
			if snap.LocalIP != "" {
				fmt.Printf(" Local IP     : %s\n", snap.LocalIP)
			}
			if snap.UptimeSeconds != nil {
				fmt.Printf(" Uptime       : %d hours\n", *snap.UptimeSeconds/3600)
			}
			if snap.UserIdleSeconds != nil && snap.UserActive != nil {
				status := "Active"
				if !*snap.UserActive {
					status = "Idle"
				}
				fmt.Printf(" User Status  : %s (%d sec idle)\n", status, *snap.UserIdleSeconds)
			}
			if snap.SessionLocked != nil {
				lockStr := "Unlocked"
				if *snap.SessionLocked {
					lockStr = "LOCKED"
				}
				fmt.Printf(" Screen Lock  : %s\n", lockStr)
			}
			if snap.MicrophoneInUse != nil {
				micStr := "Inactive"
				if *snap.MicrophoneInUse {
					micStr = "ACTIVE (On Air)"
				}
				fmt.Printf(" Microphone   : %s\n", micStr)
			}
			if snap.WebcamInUse != nil {
				camStr := "Inactive"
				if *snap.WebcamInUse {
					camStr = "ACTIVE (On Air)"
					if snap.WebcamActiveApp != "" {
						camStr += fmt.Sprintf(" [%s]", snap.WebcamActiveApp)
					}
				}
				fmt.Printf(" Webcam       : %s\n", camStr)
			}
			if snap.FullscreenActive != nil {
				fsStr := "No"
				if *snap.FullscreenActive {
					fsStr = "YES"
				}
				fmt.Printf(" Fullscreen   : %s\n", fsStr)
			}
			if snap.SystemTheme != "" {
				fmt.Printf(" Theme        : %s\n", snap.SystemTheme)
			}
			if snap.AudioOutputName != "" {
				fmt.Printf(" Audio Output : %s\n", snap.AudioOutputName)
			}
			if snap.WifiSSID != "" {
				wifiStr := snap.WifiSSID
				if snap.WifiSignalPercent != nil {
					wifiStr += fmt.Sprintf(" (%d%%)", *snap.WifiSignalPercent)
				}
				fmt.Printf(" Wi-Fi / Net  : %s\n", wifiStr)
			}
			if snap.DisplayPowered != nil {
				dispStr := "Standby / Asleep"
				if *snap.DisplayPowered {
					dispStr = "Powered On"
				}
				fmt.Printf(" Display      : %s\n", dispStr)
			}
			if snap.HasBattery && snap.BatteryPercent != nil && snap.PowerPlugged != nil {
				pwr := "Battery"
				if *snap.PowerPlugged {
					pwr = "AC Power"
				}
				fmt.Printf(" Battery      : %d %% (%s)\n", *snap.BatteryPercent, pwr)
			}
			if snap.ActiveProcess != "" {
				fmt.Printf(" Active App   : %s\n", snap.ActiveProcess)
			}
			if snap.ActiveWindowTitle != "" {
				fmt.Printf(" Window Title : %s\n", snap.ActiveWindowTitle)
			}
			if snap.UpdatePending != nil {
				reboot := "No"
				if *snap.UpdatePending {
					reboot = "YES (Restart Required)"
				}
				fmt.Printf(" Reboot Req.  : %s\n", reboot)
			}
			if snap.Media != nil {
				mediaStr := snap.Media.Status
				if snap.Media.Title != "" {
					mediaStr += fmt.Sprintf(" - %s (%s)", snap.Media.Title, snap.Media.Artist)
				}
				if snap.Media.AppID != "" {
					mediaStr += fmt.Sprintf(" [%s]", snap.Media.AppID)
				}
				fmt.Printf(" Media Player : %s\n", mediaStr)
			}
			fmt.Println("========================================")
			fmt.Println(" Press Ctrl+C to exit")
		}
	}
}

func main() {
	debug.SetGCPercent(25)
	debug.SetMemoryLimit(16 * 1024 * 1024)

	cliMode := flag.Bool("cli", false, "Run in terminal CLI/TUI mode")
	debugMode := flag.Bool("debug", false, "Run in debug mode (enables WebUI automatically with hot reload from disk)")
	headlessMode := flag.Bool("headless", false, "Run in headless background mode without system tray")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = config.DefaultConfig()
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
	if *debugMode {
		port, err := server.Start(cfg.GetPort())
		if err == nil {
			url := fmt.Sprintf("http://127.0.0.1:%d/?token=%s", port, server.Token())
			fmt.Printf("\n[DEBUG] WebUI running with live disk reload at: %s\n\n", url)
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
				_ = mqttClient.PublishTelemetry(snap)
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
