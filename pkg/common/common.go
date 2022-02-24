package common

type KubeConfig struct {
	Context string
	File    string
}

type WaitFlags struct {
	WaitForDeployments       bool
	WaitForDeploymentConfigs bool
	WaitForStatefulSets      bool
	CheckExisting            bool
}
