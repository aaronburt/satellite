//go:build !windows

package toast

import "github.com/gen2brain/beeep"

func Show(n Notification) error {
	return beeep.Notify(n.Title, n.Message, "")
}
