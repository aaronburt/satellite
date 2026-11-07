package config

func (e ExposeConfig) WithoutSensitive() ExposeConfig {
	e.UserPresence = false
	e.SessionLock = false
	e.MediaControl = false
	e.ActiveWindow = false
	e.WindowTitle = false
	e.Microphone = false
	e.Webcam = false
	e.ScreenThumbnail = false
	e.ICUERGB = false
	return e
}
