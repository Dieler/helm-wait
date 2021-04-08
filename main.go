package main

import (
	"os"

	"github.com/dieler/helm-wait/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd(os.Stdout, os.Args[1:])

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
