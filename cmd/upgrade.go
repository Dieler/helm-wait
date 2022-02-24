package cmd

import (
	"errors"
	"fmt"
	"github.com/dieler/helm-wait/pkg/common"
	"github.com/dieler/helm-wait/pkg/diff"
	"github.com/dieler/helm-wait/pkg/helm"
	"github.com/dieler/helm-wait/pkg/kube"
	"github.com/dieler/helm-wait/pkg/manifest"
	"helm.sh/helm/v3/pkg/release"
	"io"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

const upgradeCmdLongUsage = `
This command compares the current revision of the given release with its previous revision and waits until all changes of the current revision have been applied.
Example:
$ helm wait upgrade my-release
$ helm wait upgrade my-release --timeout 600
`

var (
	timeout  int64
	argFlags common.WaitFlags
)

func newUpgradeCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade RELEASE_NAME",
		Short: "Wait until all changes in the current release have been applied",
		Long:  upgradeCmdLongUsage,
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: runUpgrade,
	}

	flags := cmd.Flags()
	flags.Int64Var(&timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)")
	flags.BoolVar(&argFlags.WaitForDeployments, "wait-for-deployments", true, "Wait for Kubernetes Deployment resources")
	flags.BoolVar(&argFlags.WaitForDeploymentConfigs, "wait-for-deployment-configs", false, "Wait for Openshift's DeploymentConfig resources")
	flags.BoolVar(&argFlags.WaitForStatefulSets, "wait-for-stateful-sets", true, "Wait for Kubernetes StatefulSet resources")
	flags.BoolVar(&argFlags.CheckExisting, "check-existing", false, "Check also existing resources")
	settings.AddFlags(flags)
	return cmd
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	switch {
	case len(args) < 1:
		return errors.New("too few arguments to command \"upgrade\", the name of a release is required")
	case len(args) > 1:
		return errors.New("too many arguments to command \"upgrade\", only name of a release is allowed")
	}
	kubeConfig := common.KubeConfig{
		Context: settings.KubeContext,
		File:    settings.KubeConfigFile,
	}
	return upgrade(args[0], kubeConfig)
}

type ReleaseByVersion []*release.Release

func (o ReleaseByVersion) Len() int      { return len(o) }
func (o ReleaseByVersion) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o ReleaseByVersion) Less(i, j int) bool {
	return o[i].Version < o[j].Version
}

func upgrade(releaseName string, kubeConfig common.KubeConfig) error {
	cfg, err := helm.GetActionConfig(kubeConfig)
	if err != nil {
		return err
	}
	history, err := cfg.Releases.History(releaseName)
	if err != nil {
		return err
	}
	sort.Sort(ReleaseByVersion(history))
	currentRelease := history[len(history)-1]
	currentRelease.Info.Status.IsPending()
	if currentRelease.Info.Status.IsPending() {
		fmt.Printf("Current version is not an update or was not successful: version=%d, status=%s\n", currentRelease.Version, currentRelease.Info.Status)
		return nil
	}
	var previousRelease *release.Release
	for i := len(history) - 2; i >= 0; i-- {
		r := history[i]
		if r.Info.Status == release.StatusSuperseded {
			previousRelease = r
			break
		}
	}
	fmt.Printf("Current release: %d\n", currentRelease.Version)
	currentSpecs := manifest.ParseRelease(currentRelease, false)
	var previousSpecs map[string]*manifest.MappingResult
	if previousRelease == nil {
		previousSpecs = map[string]*manifest.MappingResult{}
	} else {
		fmt.Printf("Previous release: %d\n", previousRelease.Version)
		previousSpecs = manifest.ParseRelease(previousRelease, false)
	}

	kc, err := kube.New(os.Stdout)
	if err != nil {
		return err
	}
	if argFlags.CheckExisting {
		// wait for all resources
		fmt.Printf("Wait for all existing resources\n")
		var resources []*manifest.MappingResult
		for _, value := range currentSpecs {
			resources = append(resources, value)
		}
		return kc.WaitForResources(time.Duration(timeout)*time.Second, resources, argFlags)

	} else {
		// wait only for differences
		fmt.Printf("Wait for diffrences in resources\n")
		changes, err := diff.GetModifiedOrNewResources(previousSpecs, currentSpecs, os.Stdout)
		if err != nil {
			return err
		}
		return kc.WaitForResources(time.Duration(timeout)*time.Second, changes, argFlags)
	}
}
