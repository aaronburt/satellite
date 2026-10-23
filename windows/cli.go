package main

import (
	"fmt"
	"io"

	"satellite/internal/config"
	"satellite/internal/telemetry"
	"satellite/internal/updater"
)

// renderCLI writes one screen of the --cli dashboard. Readings that are nil
// or empty (not exposed, or not supported here) are left out.
func renderCLI(w io.Writer, snap telemetry.Snapshot, update *updater.CheckStatus) {
	fmt.Fprintf(w, "========================================\n")
	fmt.Fprintf(w, "       SATELLITE AGENT (v%s CLI)       \n", config.Version)
	fmt.Fprintf(w, "========================================\n")
	if update != nil && update.Available {
		fmt.Fprintf(w, " Update       : v%s available! (%s)\n", update.LatestVersion, update.ReleaseURL)
	}
	if snap.CPUPercent != nil {
		fmt.Fprintf(w, " Processor    : %6.1f %%\n", *snap.CPUPercent)
	}
	if snap.MemoryUsedGB != nil && snap.MemoryTotalGB != nil && snap.MemoryPercent != nil {
		fmt.Fprintf(w, " Memory (RAM) : %6.2f / %6.2f GB (%5.1f %%)\n", *snap.MemoryUsedGB, *snap.MemoryTotalGB, *snap.MemoryPercent)
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
		fmt.Fprintf(w, " GPU (%s) : %s\n", name, gpuStr)
	}
	for _, d := range snap.Drives {
		fmt.Fprintf(w, " Storage (%s) : %6.1f / %6.1f GB (%5.1f %%)\n", d.Mount, d.UsedGB, d.TotalGB, d.UsedPercent)
	}
	if snap.NetworkRxKBs != nil && snap.NetworkTxKBs != nil {
		fmt.Fprintf(w, " Network I/O  : ↓ %7.1f kB/s | ↑ %7.1f kB/s\n", *snap.NetworkRxKBs, *snap.NetworkTxKBs)
	}
	if snap.LocalIP != "" {
		fmt.Fprintf(w, " Local IP     : %s\n", snap.LocalIP)
	}
	if snap.UptimeSeconds != nil {
		fmt.Fprintf(w, " Uptime       : %d hours\n", *snap.UptimeSeconds/3600)
	}
	if snap.UserIdleSeconds != nil && snap.UserActive != nil {
		status := "Active"
		if !*snap.UserActive {
			status = "Idle"
		}
		fmt.Fprintf(w, " User Status  : %s (%d sec idle)\n", status, *snap.UserIdleSeconds)
	}
	if snap.SessionLocked != nil {
		lockStr := "Unlocked"
		if *snap.SessionLocked {
			lockStr = "LOCKED"
		}
		fmt.Fprintf(w, " Screen Lock  : %s\n", lockStr)
	}
	if snap.MicrophoneInUse != nil {
		micStr := "Inactive"
		if *snap.MicrophoneInUse {
			micStr = "ACTIVE (On Air)"
		}
		fmt.Fprintf(w, " Microphone   : %s\n", micStr)
	}
	if snap.WebcamInUse != nil {
		camStr := "Inactive"
		if *snap.WebcamInUse {
			camStr = "ACTIVE (On Air)"
			if snap.WebcamActiveApp != "" {
				camStr += fmt.Sprintf(" [%s]", snap.WebcamActiveApp)
			}
		}
		fmt.Fprintf(w, " Webcam       : %s\n", camStr)
	}
	if snap.FullscreenActive != nil {
		fsStr := "No"
		if *snap.FullscreenActive {
			fsStr = "YES"
		}
		fmt.Fprintf(w, " Fullscreen   : %s\n", fsStr)
	}
	if snap.SystemTheme != "" {
		fmt.Fprintf(w, " Theme        : %s\n", snap.SystemTheme)
	}
	if snap.AudioOutputName != "" {
		fmt.Fprintf(w, " Audio Output : %s\n", snap.AudioOutputName)
	}
	if snap.WifiSSID != "" {
		wifiStr := snap.WifiSSID
		if snap.WifiSignalPercent != nil {
			wifiStr += fmt.Sprintf(" (%d%%)", *snap.WifiSignalPercent)
		}
		fmt.Fprintf(w, " Wi-Fi / Net  : %s\n", wifiStr)
	}
	if snap.DisplayPowered != nil {
		dispStr := "Standby / Asleep"
		if *snap.DisplayPowered {
			dispStr = "Powered On"
		}
		fmt.Fprintf(w, " Display      : %s\n", dispStr)
	}
	if snap.HasBattery && snap.BatteryPercent != nil && snap.PowerPlugged != nil {
		pwr := "Battery"
		if *snap.PowerPlugged {
			pwr = "AC Power"
		}
		fmt.Fprintf(w, " Battery      : %d %% (%s)\n", *snap.BatteryPercent, pwr)
	}
	if snap.ActiveProcess != "" {
		fmt.Fprintf(w, " Active App   : %s\n", snap.ActiveProcess)
	}
	if snap.ActiveWindowTitle != "" {
		fmt.Fprintf(w, " Window Title : %s\n", snap.ActiveWindowTitle)
	}
	if snap.UpdatePending != nil {
		reboot := "No"
		if *snap.UpdatePending {
			reboot = "YES (Restart Required)"
		}
		fmt.Fprintf(w, " Reboot Req.  : %s\n", reboot)
	}
	if snap.Media != nil {
		mediaStr := snap.Media.Status
		if snap.Media.Title != "" {
			mediaStr += fmt.Sprintf(" - %s (%s)", snap.Media.Title, snap.Media.Artist)
		}
		if snap.Media.AppID != "" {
			mediaStr += fmt.Sprintf(" [%s]", snap.Media.AppID)
		}
		fmt.Fprintf(w, " Media Player : %s\n", mediaStr)
	}
	fmt.Fprintln(w, "========================================")
	fmt.Fprintln(w, " Press Ctrl+C to exit")
}
