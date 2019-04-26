package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version identifier populated via the CI/CD process.
var Version = "HEAD"

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version of the helm wait plugin",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
