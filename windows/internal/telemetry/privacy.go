package telemetry

import "satellite/internal/privacy"

func (s Snapshot) ApplyPrivacy() Snapshot {
	status := privacy.Default.Status()
	if status.Paused || (s.privacyGeneration != 0 && s.privacyGeneration != status.Generation) {
		s = s.withoutSensitive()
	}
	s.privacyGeneration = status.Generation
	return s
}

func (s Snapshot) withoutSensitive() Snapshot {
	s.UserIdleSeconds = nil
	s.UserActive = nil
	s.SessionLocked = nil
	s.Media = nil
	if s.SummaryThumbnailAvailable != nil {
		unavailable := false
		s.SummaryThumbnailAvailable = &unavailable
	}
	s.RGB = nil
	s.ActiveProcess = ""
	s.ActiveWindowTitle = ""
	s.MicrophoneInUse = nil
	s.WebcamInUse = nil
	s.WebcamActiveApp = ""
	return s
}

func (c *Collector) ClearSensitive() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSnapshot = c.lastSnapshot.withoutSensitive()
}
