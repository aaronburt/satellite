package webui

import (
	"regexp"
	"strings"
	"testing"
)

func TestSettingsPageScreenThumbnailToggle(t *testing.T) {
	page := string(indexHTML)
	label := regexp.MustCompile(`<label[^>]*data-cap="screen_thumbnail"[^>]*>\s*<input[^>]*id="exp-screen-thumb"[^>]*>`)
	tag := label.FindString(page)
	if tag == "" {
		t.Fatal("no screen thumbnail toggle gated on the screen_thumbnail capability")
	}
	if !strings.Contains(tag, "data-no-bulk") {
		t.Error("screen thumbnail toggle must be excluded from bulk enable")
	}
	if !strings.Contains(page, "screen_thumbnail: $('exp-screen-thumb').checked") {
		t.Error("toggle is not saved as expose.screen_thumbnail")
	}
	if !strings.Contains(page, "$('exp-screen-thumb').checked = !!currentExpose.screen_thumbnail") {
		t.Error("toggle does not load expose.screen_thumbnail")
	}
	if !strings.Contains(page, "if (checked && el.dataset.noBulk !== undefined) return;") {
		t.Error("bulk toggle no longer skips data-no-bulk switches")
	}
}
