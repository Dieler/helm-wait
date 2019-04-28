package cmd

import (
	"errors"
	"fmt"
	"github.com/dieler/helm-wait/pkg/kube"
	"github.com/dieler/helm-wait/pkg/manifest"
	"os"
	"time"

	"github.com/mgutz/ansi"
	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/release"
)

type upgrade struct {
	release          string
	client           helm.Interface
}

const upgradeCmdLongUsage = `
This command compares the current revision of the given release with its previous revision and waits until all changes of the current revision have been applied.

Example:
$ helm wait upgrade my-release
`

// upgradeCmd waits until all added resources have been created and all changes have been applied.
var (
	upgradeCmd = &cobra.Command{
		Use:   "upgrade",
		Short: "Wait until all changes in the current release have been applied",
		Long:  upgradeCmdLongUsage,
		PreRun: func(*cobra.Command, []string) {
			expandTLSPaths()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// suppress the command usage on error
			cmd.SilenceUsage = true
			upgrade := upgrade{}
			switch {
			case len(args) < 1:
				return errors.New("too few arguments to command \"upgrade\", the name of a release is required")
			case len(args) > 1:
				return errors.New("too many arguments to command \"upgrade\", only name of a release is allowed")
			}
			upgrade.release = args[0]
			if upgrade.client == nil {
				upgrade.client = createHelmClient()
			}
			return upgrade.run()
		},
	}
	timeout int64
)

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().Int64Var(&timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)")
	addCommonCmdOptions(upgradeCmd.Flags())
}

func (u *upgrade) run() error {
	releaseHistory, err := u.client.ReleaseHistory(u.release, helm.WithMaxHistory(5))
	if err != nil {
		return prettyError(err)
	}
	releases := releaseHistory.GetReleases()
	currentRelease := releases[0]
	status := currentRelease.GetInfo().GetStatus().Code
	if status != release.Status_DEPLOYED && status != release.Status_PENDING_UPGRADE {
		fmt.Printf("Current version is not an update or was not successful: version=%d, status=%s\n", currentRelease.GetVersion(), status)
		return nil
	}
	var previousRelease *release.Release
	for _, r := range releases[1:] {
		if r.GetInfo().GetStatus().Code == release.Status_SUPERSEDED {
			previousRelease = r
			break
		}
	}
	fmt.Printf("Current release: %d\n", currentRelease.GetVersion())
	currentSpecs := manifest.ParseRelease(currentRelease, false)
	var previousSpecs map[string]*manifest.MappingResult
	if previousRelease == nil {
		previousSpecs = map[string]*manifest.MappingResult{}
	} else {
		fmt.Printf("Previous release: %d\n", previousRelease.GetVersion())
		previousSpecs = manifest.ParseRelease(previousRelease, false)
	}
	changes, err := u.getModifiedOrNewResources(previousSpecs, currentSpecs)
	if err != nil {
		return err
	}
	kc, err := kube.New()
	if err != nil {
		return err
	}
	return kc.WaitForResources(time.Duration(timeout)*time.Second, changes)
}


type change int
const (
	ADDED change = iota
	CHANGED
	REMOVED
)

func (c change) color() string {
	return [...]string{"green", "yellow", "red"}[c]
}

func (c change) format() string {
	return [...]string{"++ %s", "~~ %s", "-- %s"}[c]
}

func (u *upgrade) getModifiedOrNewResources(previous, current map[string]*manifest.MappingResult) ([]*manifest.MappingResult, error) {
	result := []*manifest.MappingResult{}
	changes := make(map[string]change)
	for key, previousValue := range previous {
		if currentValue, ok := current[key]; ok {
			if previousValue.Content != currentValue.Content {
				changes[key] = CHANGED
				result = append(result, currentValue)
			}
		} else {
			changes[key] = REMOVED
		}
	}
	for key := range current {
		if _, ok := previous[key]; !ok {
			changes[key] = ADDED
			result = append(result, current[key])
		}
	}
	if len(changes) > 0 {
		fmt.Println("Changes:")
		for k, v := range changes {
			u.fprintf(v.color(), v.format(), k)
		}
	} else {
		fmt.Println("No changes")
	}
	return result, nil
}

func (u *upgrade) fprintf(color, format string, args ...interface{}) {
	if _, err := fmt.Fprintf(os.Stdout, ansi.Color(format, color)+"\n", args); err != nil {
		// do nothing else, just stop Intellij complaining about unhandled errors
		return
	}
}
