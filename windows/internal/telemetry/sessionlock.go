package telemetry

import "encoding/binary"

// Lock detection on Windows asks the session manager directly via
// WTSQuerySessionInformation(WTSSessionInfoEx). The helpers here hold the
// parsing and decision logic so it can be tested on any OS.

const (
	wtsSessionStateLock    int32 = 0 // WTS_SESSIONSTATE_LOCK
	wtsSessionStateUnlock  int32 = 1 // WTS_SESSIONSTATE_UNLOCK
	wtsSessionStateUnknown int32 = -1

	// Windows 8 is build 9200. On Windows 7 / Server 2008 R2 the meaning of
	// SessionFlags is inverted (a documented bug in WTSINFOEX_LEVEL1).
	firstBuildWithCorrectWTSFlags = 9200

	// Byte offset of SessionFlags inside WTSINFOEXW: Level (4 bytes), then
	// the union aligned to 8 (it holds LARGE_INTEGERs), then SessionId (4)
	// and SessionState (4).
	wtsInfoExSessionFlagsOffset = 16
)

// wtsSessionFlagsFromInfoEx reads SessionFlags out of a raw WTSINFOEXW
// buffer. ok is false when the buffer is too short or not level 1.
func wtsSessionFlagsFromInfoEx(buf []byte) (flags int32, ok bool) {
	if len(buf) < wtsInfoExSessionFlagsOffset+4 {
		return 0, false
	}
	if binary.LittleEndian.Uint32(buf[0:4]) != 1 {
		return 0, false
	}
	return int32(binary.LittleEndian.Uint32(buf[wtsInfoExSessionFlagsOffset:])), true
}

// lockStateFromWTSFlags turns SessionFlags into a lock state. known is false
// when Windows reports the state as unknown or returns an unexpected value.
func lockStateFromWTSFlags(flags int32, osBuild uint32) (locked, known bool) {
	if osBuild != 0 && osBuild < firstBuildWithCorrectWTSFlags {
		switch flags {
		case wtsSessionStateLock:
			flags = wtsSessionStateUnlock
		case wtsSessionStateUnlock:
			flags = wtsSessionStateLock
		}
	}
	switch flags {
	case wtsSessionStateLock:
		return true, true
	case wtsSessionStateUnlock:
		return false, true
	default:
		return false, false
	}
}

// resolveSessionLocked picks the answer to report. The WTS state wins when
// it is known. Otherwise it falls back to whether the input desktop can be
// opened, which is the older and less reliable signal.
func resolveSessionLocked(wtsLocked, wtsKnown, inputDesktopOpenable bool) bool {
	if wtsKnown {
		return wtsLocked
	}
	return !inputDesktopOpenable
}
