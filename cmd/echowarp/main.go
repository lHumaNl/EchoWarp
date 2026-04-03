// Package main is the entry point for the EchoWarp application.
package main

import (
	"fmt"
	"os"

	"github.com/lHumaNl/echowarp/internal/cli"
)

func main() {
	rootCmd := cli.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
