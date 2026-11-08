//go:build !windows

package main

import (
	"fmt"
	"os"
)

func runUIClient() {
	fmt.Fprintln(os.Stderr, "The dedicated UI client is only available on Windows; open the debug UI URL in a browser.")
	os.Exit(1)
}
