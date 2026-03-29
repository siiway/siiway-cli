package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "siiway",
	Short: "SiiWay CLI",
	Long:  "SiiWay CLI provides project scaffolding and developer workflows.",
}

// Execute runs the root CLI command and exits with non-zero status on failure.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
