package kube

import (
	"context"
	"fmt"
	"github.com/dieler/helm-wait/pkg/manifest"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
	"sort"
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
		statefulSets := []appsv1.StatefulSet{}
		deployments := []deployment{}
		for _, r := range resources {
			switch r.Metadata.Kind {
			case "ConfigMap":
			case "Service":
			case "ReplicationController":
			case "Pod":
			case "Deployment":
				currentDeployment, err := c.clientset.AppsV1().Deployments(r.Metadata.Metadata.Namespace).Get(context.TODO(), r.Metadata.Metadata.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				// Find RS associated with deployment
				newReplicaSet, err := c.getNewReplicaSet(currentDeployment)
				if err != nil || newReplicaSet == nil {
					return false, err
				}
				newDeployment := deployment{
					newReplicaSet,
					currentDeployment,
				}
				deployments = append(deployments, newDeployment)
			case "StatefulSet":
				sf, err := c.clientset.AppsV1().StatefulSets(r.Metadata.Metadata.Namespace).Get(context.TODO(), r.Metadata.Metadata.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				statefulSets = append(statefulSets, *sf)
			}
		}
		isReady := c.statefulSetsReady(statefulSets) && c.deploymentsReady(deployments)
		return isReady, nil
	})
}

// GetNewReplicaSet returns a replica set that matches the intent of the given deployment; get ReplicaSetList from client interface.
// Returns nil if the new replica set doesn't exist yet.
func (c *Client) getNewReplicaSet(deployment *appsv1.Deployment) (*appsv1.ReplicaSet, error) {
	rsList, err := c.listReplicaSets(deployment, c.rsListFromClient())
	if err != nil {
		return nil, err
	}
	return findNewReplicaSet(deployment, rsList), nil
}

// RsListFunc returns the ReplicaSet from the ReplicaSet namespace and the List metav1.ListOptions.
type RsListFunc func(string, metav1.ListOptions) ([]*appsv1.ReplicaSet, error)

// ListReplicaSets returns a slice of RSes the given deployment targets.
// Note that this does NOT attempt to reconcile ControllerRef (adopt/orphan),
// because only the controller itself should do that.
// However, it does filter out anything whose ControllerRef doesn't match.
func (c *Client) listReplicaSets(deployment *appsv1.Deployment, getRSList RsListFunc) ([]*appsv1.ReplicaSet, error) {
	// TODO: Right now we list replica sets by their labels. We should list them by selector, i.e. the replica set's selector
	//       should be a superset of the deployment's selector, see https://github.com/kubernetes/kubernetes/issues/19830.
	namespace := deployment.Namespace
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}
	options := metav1.ListOptions{LabelSelector: selector.String()}
	all, err := getRSList(namespace, options)
	if err != nil {
		return nil, err
	}
	// Only include those whose ControllerRef matches the Deployment.
	owned := make([]*appsv1.ReplicaSet, 0, len(all))
	for _, rs := range all {
		if metav1.IsControlledBy(rs, deployment) {
			owned = append(owned, rs)
		}
	}
	return owned, nil
}

// RsListFromClient returns an rsListFunc that wraps the given client.
func (c *Client) rsListFromClient() RsListFunc {
	return func(namespace string, options metav1.ListOptions) ([]*appsv1.ReplicaSet, error) {
		rsList, err := c.clientset.AppsV1().ReplicaSets(namespace).List(context.TODO(), options)
		if err != nil {
			return nil, err
		}
		var ret []*appsv1.ReplicaSet
		for i := range rsList.Items {
			ret = append(ret, &rsList.Items[i])
		}
		return ret, err
	}
}

// EqualIgnoreHash returns true if two given podTemplateSpec are equal, ignoring the diff in value of Labels[pod-template-hash]
// We ignore pod-template-hash because:
// 1. The hash result would be different upon podTemplateSpec API changes
//    (e.g. the addition of a new field will cause the hash code to change)
// 2. The deployment template won't have hash labels
func EqualIgnoreHash(template1, template2 *v1.PodTemplateSpec) bool {
	t1Copy := template1.DeepCopy()
	t2Copy := template2.DeepCopy()
	// Remove hash labels from template.Labels before comparing
	delete(t1Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	delete(t2Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	return apiequality.Semantic.DeepEqual(t1Copy, t2Copy)
}

// ReplicaSetsByCreationTimestamp sorts a list of ReplicaSet by creation timestamp, using their names as a tie breaker.
type ReplicaSetsByCreationTimestamp []*appsv1.ReplicaSet

func (o ReplicaSetsByCreationTimestamp) Len() int      { return len(o) }
func (o ReplicaSetsByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o ReplicaSetsByCreationTimestamp) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}

// FindNewReplicaSet returns the new RS this given deployment targets (the one with the same pod template).
func findNewReplicaSet(deployment *appsv1.Deployment, rsList []*appsv1.ReplicaSet) *appsv1.ReplicaSet {
	sort.Sort(ReplicaSetsByCreationTimestamp(rsList))
	for i := range rsList {
		if EqualIgnoreHash(&rsList[i].Spec.Template, &deployment.Spec.Template) {
			// In rare cases, such as after cluster upgrades, Deployment may end up with
			// having more than one new ReplicaSets that have the same template as its template,
			// see https://github.com/kubernetes/kubernetes/issues/40415
			// We deterministically choose the oldest new ReplicaSet.
			return rsList[i]
		}
	}
	// new ReplicaSet does not exist.
	return nil
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
