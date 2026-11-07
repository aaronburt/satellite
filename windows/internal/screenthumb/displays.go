package screenthumb

import (
	"fmt"
	"slices"
)

const MaxDisplays = 64

type Display struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Device  string `json:"device"`
	Primary bool   `json:"primary"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

func Displays() ([]Display, error) {
	return platformDisplays()
}

func NormalizeDisplays(ids []int) []int {
	var out []int
	for _, id := range ids {
		if id > 0 && id <= MaxDisplays && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

func ValidateDisplays(ids []int) error {
	for _, id := range ids {
		if id < 1 || id > MaxDisplays {
			return fmt.Errorf("screen_thumbnail_displays must contain display numbers from 1 to %d", MaxDisplays)
		}
	}
	return nil
}

func SelectedDisplays(ids []int) []int {
	if normalized := NormalizeDisplays(ids); len(normalized) > 0 {
		return normalized
	}
	return []int{0}
}

func DisplaySelected(ids []int, id int) bool {
	return slices.Contains(SelectedDisplays(ids), id)
}

func DisplayKey(id int) string {
	if id == 0 {
		return "screen_thumbnail"
	}
	return fmt.Sprintf("screen_thumbnail_display_%d", id)
}

func ResolveDisplay(displays []Display, id int) (Display, bool) {
	for _, display := range displays {
		if (id == 0 && display.Primary) || display.ID == id {
			display.ID = id
			return display, true
		}
	}
	return Display{ID: id}, false
}
