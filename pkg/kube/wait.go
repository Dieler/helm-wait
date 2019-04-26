package kube

import (
	"fmt"
	"github.com/dieler/helm-wait/pkg/manifest"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"
	"path/filepath"
	"time"
)

var Config = filepath.Join(homedir.HomeDir(), ".kube", "config")

type Client struct {
	clientset *kubernetes.Clientset
}

func New() (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", Config)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{clientset: clientset}, nil
}

// deployment holds associated replicaSet for a deployment
type deployment struct {
	replicaSet *appsv1.ReplicaSet
	deployment *appsv1.Deployment
}

// WaitForResources polls to get the current status of all deployments and stateful sets
// until they are ready or a timeout is reached
func (c *Client) WaitForResources(timeout time.Duration, resources []*manifest.MappingResult) error {
	return wait.Poll(5*time.Second, timeout, func() (bool, error) {
		statefulsets := []appsv1.StatefulSet{}
		deployments := []deployment{}
		for _, r := range resources {
			switch r.Metadata.TypeString() {
			case "v1.ConfigMap":
			case "v1.Service":
			case "v1.ReplicationController":
			case "v1.Pod":
			case "appsv1.Deployment", "appsv1beta1.Deployment", "appsv1beta2.Deployment", "extensions.Deployment":
				currentDeployment, err := c.clientset.AppsV1().Deployments(r.Metadata.Metadata.Namespace).Get(r.Metadata.Metadata.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				// Find RS associated with deployment
				newReplicaSet, err := deploymentutil.GetNewReplicaSet(currentDeployment, c.clientset.AppsV1())
				if err != nil || newReplicaSet == nil {
					return false, err
				}
				newDeployment := deployment{
					newReplicaSet,
					currentDeployment,
				}
				deployments = append(deployments, newDeployment)
			case "appsv1.StatefulSet", "appsv1beta1.StatefulSet", "appsv1beta2.StatefulSet":
				sf, err := c.clientset.AppsV1().StatefulSets(r.Metadata.Metadata.Namespace).Get(r.Metadata.Metadata.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				statefulsets = append(statefulsets, *sf)
			}
		}
		isReady := c.statefulSetsReady(statefulsets) && c.deploymentsReady(deployments)
		return isReady, nil
	})
}

func (c *Client) statefulSetsReady(statefulsets []appsv1.StatefulSet) bool {
	for _, sf := range statefulsets {
		if sf.Status.UpdateRevision != sf.Status.CurrentRevision || sf.Status.ReadyReplicas != *sf.Spec.Replicas {
			fmt.Printf("StatefulSet is not ready: %s/%s\n", sf.GetNamespace(), sf.GetName())
			return false
		}
	}
	return true
}

func (c *Client) deploymentsReady(deployments []deployment) bool {
	for _, d := range deployments {
		if d.replicaSet.Status.ReadyReplicas != *d.deployment.Spec.Replicas {
			fmt.Printf("Deployment is not ready: %s/%s\n", d.deployment.GetNamespace(), d.deployment.GetName())
			return false
		}
	}
	return true
}
