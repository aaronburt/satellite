//go:build windows

package main

import (
	"satellite/internal/mqtt"
	"satellite/internal/tray"
	"satellite/internal/updater"
	"satellite/internal/webui"
)

func runTray(server *webui.Server, mqttClient *mqtt.Client, updaterInstance *updater.Checker, onExit func()) {
	t := tray.NewTray(server, mqttClient, updaterInstance, onExit)
	t.Run()
}
