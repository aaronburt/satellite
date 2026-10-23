package main

import (
	"fmt"
	"io"
	"os"

	"satellite/internal/config"
)

// runConfigCommand handles the -export and -import flags. handled is true when
// one of them was given (the program should exit with code afterwards).
func runConfigCommand(cfg config.Config, exportPath, importPath, passphrase string, resetNodeID bool, stdout, stderr io.Writer) (handled bool, code int) {
	if exportPath != "" {
		data, err := config.ExportConfig(cfg, passphrase)
		if err != nil {
			fmt.Fprintf(stderr, "Error exporting configuration: %v\n", err)
			return true, 1
		}
		if err := os.WriteFile(exportPath, data, 0600); err != nil {
			fmt.Fprintf(stderr, "Error writing exported file: %v\n", err)
			return true, 1
		}
		fmt.Fprintf(stdout, "Configuration exported successfully to %s\n", exportPath)
		return true, 0
	}

	if importPath != "" {
		raw, err := os.ReadFile(importPath)
		if err != nil {
			fmt.Fprintf(stderr, "Error reading import file: %v\n", err)
			return true, 1
		}
		res, err := config.ImportConfig(raw, config.ImportOptions{
			Passphrase:  passphrase,
			ResetNodeID: resetNodeID,
		})
		if err != nil {
			fmt.Fprintf(stderr, "Error importing configuration: %v\n", err)
			return true, 1
		}
		if err := config.Save(res.Config); err != nil {
			fmt.Fprintf(stderr, "Error saving imported configuration: %v\n", err)
			return true, 1
		}
		fmt.Fprintf(stdout, "Configuration imported successfully from Satellite v%s (Active Node ID: %s)\n", res.SourceAppVersion, res.Config.NodeID)
		if res.IsNewerVersion {
			fmt.Fprintf(stdout, "Notice: Configuration was created by a newer version of Satellite (v%s)\n", res.SourceAppVersion)
		}
		return true, 0
	}

	return false, 0
}
