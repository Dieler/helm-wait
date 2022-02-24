package helm

import (
	"fmt"
	"log"
	"os"

	"github.com/dieler/helm-wait/pkg/common"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
)

var (
	settings = cli.New()
)

// GetActionConfig returns action configuration based on Helm env
func GetActionConfig(kubeConfig common.KubeConfig) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)

	// Add kube config settings passed by user
	settings.KubeConfig = kubeConfig.File
	settings.KubeContext = kubeConfig.Context

	err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), debug)
	if err != nil {
		return nil, err
	}

	return actionConfig, err
}

func debug(format string, v ...interface{}) {
	if settings.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		log.Output(2, fmt.Sprintf(format, v...))
	}
}
