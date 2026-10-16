package updater

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected Version
	}{
		{"0.14.0", Version{0, 14, 0}},
		{"v0.14.0", Version{0, 14, 0}},
		{"V1.2.3", Version{1, 2, 3}},
		{"v2.0.0-beta.1", Version{2, 0, 0}},
		{"1.0.0+build123", Version{1, 0, 0}},
		{"  v0.15.1  ", Version{0, 15, 1}},
		{"invalid", Version{0, 0, 0}},
	}

	for _, tc := range tests {
		result := ParseVersion(tc.input)
		if result != tc.expected {
			t.Errorf("ParseVersion(%q) = %+v, expected %+v", tc.input, result, tc.expected)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"0.14.0", "0.14.0", 0},
		{"v0.14.0", "0.14.0", 0},
		{"0.15.0", "0.14.0", 1},
		{"0.14.0", "0.15.0", -1},
		{"1.0.0", "0.99.99", 1},
		{"0.14.1", "0.14.0", 1},
		{"0.14.0", "0.14.1", -1},
		{"v1.2.3", "v1.2.4", -1},
	}

	for _, tc := range tests {
		result := CompareVersions(tc.v1, tc.v2)
		if result != tc.expected {
			t.Errorf("CompareVersions(%q, %q) = %d, expected %d", tc.v1, tc.v2, result, tc.expected)
		}
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("0.14.0", "0.15.0") {
		t.Errorf("expected 0.15.0 to be newer than 0.14.0")
	}
	if !IsNewer("v0.14.0", "v0.14.1") {
		t.Errorf("expected v0.14.1 to be newer than v0.14.0")
	}
	if IsNewer("0.15.0", "0.14.0") {
		t.Errorf("expected 0.14.0 not to be newer than 0.15.0")
	}
	if IsNewer("0.14.0", "0.14.0") {
		t.Errorf("expected 0.14.0 not to be newer than 0.14.0")
	}
}
