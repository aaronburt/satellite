//go:build !windows

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"satellite/internal/mqtt"
	"satellite/internal/updater"
	"satellite/internal/webui"
)

func runTray(server *webui.Server, mqttClient *mqtt.Client, updaterInstance *updater.Checker, onExit func()) {
	sigCtx, sigCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer sigCancel()
	<-sigCtx.Done()
	onExit()
}
