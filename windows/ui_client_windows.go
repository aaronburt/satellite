package main

import (
	"fmt"
	"os"

	"satellite/internal/desktopui"
)

func runUIClient() {
	if err := desktopui.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
