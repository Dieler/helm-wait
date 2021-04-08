package cmd

import (
	"github.com/spf13/pflag"
)

type EnvSettings struct {
	namespace      string
	KubeConfigFile string
	KubeContext    string
}

func New() *EnvSettings {
	envSettings := EnvSettings{}
	return &envSettings
}

// AddBaseFlags binds base flags to the given flagset.
func (s *EnvSettings) AddBaseFlags(fs *pflag.FlagSet) {
}

// AddFlags binds flags to the given flagset.
func (s *EnvSettings) AddFlags(fs *pflag.FlagSet) {
	s.AddBaseFlags(fs)
	fs.StringVarP(&s.namespace, "namespace", "n", s.namespace, "namespace scope for this request")
	fs.StringVar(&s.KubeConfigFile, "kubeconfig", "", "path to the kubeconfig file")
	fs.StringVar(&s.KubeContext, "kube-context", s.KubeContext, "name of the kubeconfig context to use")
}
