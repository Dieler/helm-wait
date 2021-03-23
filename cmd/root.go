package cmd

import (
	"errors"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var (
	settings *EnvSettings
)

func NewRootCmd(out io.Writer, args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "wait",
		Short:        "helm wait plugin",
		Long:         "helm wait plugin",
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return errors.New("no arguments accepted")
			}
			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.Parse(args)
	settings = new(EnvSettings)

	if ctx := os.Getenv("HELM_KUBECONTEXT"); ctx != "" {
		settings.KubeContext = ctx
	}

	cmd.AddCommand(
		newUpgradeCmd(out),
	)

	return cmd
}
