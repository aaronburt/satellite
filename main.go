//go:build windows

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

	"satellite/internal/config"
	"satellite/internal/mqtt"
	"satellite/internal/telemetry"
	"satellite/internal/tray"
	"satellite/internal/webui"
)

func runCLI(collector *telemetry.Collector, cfg config.Config) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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
			if snap.CPUPercent != nil {
				fmt.Printf(" Processor    : %6.1f %%\n", *snap.CPUPercent)
			}
			if snap.MemoryUsedGB != nil && snap.MemoryTotalGB != nil && snap.MemoryPercent != nil {
				fmt.Printf(" Memory (RAM) : %6.2f / %6.2f GB (%5.1f %%)\n", *snap.MemoryUsedGB, *snap.MemoryTotalGB, *snap.MemoryPercent)
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
			if snap.WindowsTheme != "" {
				fmt.Printf(" Theme        : %s\n", snap.WindowsTheme)
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
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	collector := telemetry.NewCollector()

	if *cliMode {
		runCLI(collector, cfg)
		return
	}

	initialSnap := collector.Collect(cfg.Expose)
	telemetry.TrimWorkingSet()

	mqttClient := mqtt.NewClient()
	mqttClient.SetHasBattery(initialSnap.HasBattery)
	mqttClient.Start(cfg)

	server := webui.NewServer(collector, mqttClient)
	server.SetWebUIEnabled(false)
	if cfg.JSONEnabled {
		if cfg.APIKey == "" {
			cfg.APIKey = config.GenerateAPIKey()
			_ = config.Save(cfg)
		}
		server.SetJSONEnabled(true)
		server.SetAPIKey(cfg.APIKey)
		_, _ = server.Start(cfg.WebUIPort)
	}

	tickerCtx, cancelTicker := context.WithCancel(context.Background())
	go func() {
		interval := time.Duration(cfg.IntervalSec) * time.Second
		if interval < 1*time.Second {
			interval = 5 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-tickerCtx.Done():
				return
			case <-ticker.C:
				currentCfg := config.Get()
				snap := collector.Collect(currentCfg.Expose)
				_ = mqttClient.PublishTelemetry(snap)
			}
		}
	}()

	onExit := func() {
		cancelTicker()
		mqttClient.Stop()
		server.Stop()
	}

	t := tray.NewTray(server, mqttClient, onExit)
	t.Run()
}
