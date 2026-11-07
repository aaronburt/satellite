package telemetry

import (
	"encoding/binary"
	"testing"
)

// infoEx builds a WTSINFOEXW buffer the way Windows lays it out on x64.
func infoEx(level uint32, sessionID uint32, state uint32, flags int32) []byte {
	buf := make([]byte, 64)
	binary.LittleEndian.PutUint32(buf[0:], level)
	binary.LittleEndian.PutUint32(buf[8:], sessionID)
	binary.LittleEndian.PutUint32(buf[12:], state)
	binary.LittleEndian.PutUint32(buf[16:], uint32(flags))
	return buf
}

func TestWTSSessionFlagsFromInfoEx(t *testing.T) {
	cases := []struct {
		name      string
		buf       []byte
		wantFlags int32
		wantOK    bool
	}{
		{"locked", infoEx(1, 2, 0, wtsSessionStateLock), wtsSessionStateLock, true},
		{"unlocked", infoEx(1, 2, 0, wtsSessionStateUnlock), wtsSessionStateUnlock, true},
		{"unknown is negative", infoEx(1, 2, 0, -1), -1, true},
		// SessionId and SessionState must not be mistaken for the flags.
		{"ignores neighbouring fields", infoEx(1, 0xFFFFFFFF, 4, wtsSessionStateUnlock), wtsSessionStateUnlock, true},
		{"wrong level", infoEx(2, 2, 0, wtsSessionStateLock), 0, false},
		{"too short", make([]byte, 19), 0, false},
		{"nil", nil, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags, ok := wtsSessionFlagsFromInfoEx(tc.buf)
			if flags != tc.wantFlags || ok != tc.wantOK {
				t.Fatalf("got (%d, %v), want (%d, %v)", flags, ok, tc.wantFlags, tc.wantOK)
			}
		})
	}
}

func TestLockStateFromWTSFlags(t *testing.T) {
	const win7, win8, win11 = 7601, 9200, 26100
	cases := []struct {
		name       string
		flags      int32
		build      uint32
		wantLocked bool
		wantKnown  bool
	}{
		{"win11 locked", wtsSessionStateLock, win11, true, true},
		{"win11 unlocked", wtsSessionStateUnlock, win11, false, true},
		{"win8 is the first build with correct flags", wtsSessionStateLock, win8, true, true},
		// Windows 7 reports these the wrong way round.
		{"win7 lock flag means unlocked", wtsSessionStateLock, win7, false, true},
		{"win7 unlock flag means locked", wtsSessionStateUnlock, win7, true, true},
		{"unknown build trusts flags as documented", wtsSessionStateLock, 0, true, true},
		{"unknown state", wtsSessionStateUnknown, win11, false, false},
		{"unknown state on win7 stays unknown", wtsSessionStateUnknown, win7, false, false},
		{"unexpected value", 7, win11, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			locked, known := lockStateFromWTSFlags(tc.flags, tc.build)
			if locked != tc.wantLocked || known != tc.wantKnown {
				t.Fatalf("got (%v, %v), want (%v, %v)", locked, known, tc.wantLocked, tc.wantKnown)
			}
		})
	}
}

func TestResolveSessionLocked(t *testing.T) {
	cases := []struct {
		name                        string
		wtsLocked, wtsKnown, deskOK bool
		want                        bool
	}{
		// The original bug: Windows 10/11 lock screen leaves the input
		// desktop openable, so the desktop check alone said "unlocked".
		{"WTS locked beats openable desktop", true, true, true, true},
		{"WTS unlocked beats unopenable desktop", false, true, false, false},
		{"WTS locked and desktop agrees", true, true, false, true},
		{"fallback: desktop not openable means locked", false, false, false, true},
		{"fallback: desktop openable means unlocked", false, false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveSessionLocked(tc.wtsLocked, tc.wtsKnown, tc.deskOK); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
