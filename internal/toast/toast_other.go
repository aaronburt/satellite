//go:build !windows

package toast

import "errors"

func Show(n Notification) error {
	return errors.New("toast notifications are only supported on Windows")
}
