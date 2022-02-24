package kube

import (
	"context"
	"fmt"
	"github.com/dieler/helm-wait/pkg/common"
	"github.com/dieler/helm-wait/pkg/manifest"
	"io"
	"k8s.io/apimachinery/pkg/labels"
	"path/filepath"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//kccorev1     "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	oappsv1 "github.com/openshift/api/apps/v1"
	oappsv1client "github.com/openshift/client-go/apps/clientset/versioned"
)

var Config = filepath.Join(homedir.HomeDir(), ".kube", "config")

type Client struct {
	kubeclientset *kubernetes.Clientset
	occlientset   *oappsv1client.Clientset
	out           io.Writer
}

func New(out io.Writer) (*Client, error) {
	restconfig, err := clientcmd.BuildConfigFromFlags("", Config)
	if err != nil {
		return nil, err
	}
	kubeclientset, err := kubernetes.NewForConfig(restconfig)
	if err != nil {
		return nil, err
	}
	occlientset, err := oappsv1client.NewForConfig(restconfig)
	if err != nil {
		return nil, err
	}
	return &Client{kubeclientset: kubeclientset, occlientset: occlientset, out: out}, nil
}

// deployment holds associated replicaSet for a deployment
type deployment struct {
	rs *appsv1.ReplicaSet
	d  *appsv1.Deployment
}

type deploymentConfig struct {
	rc *kcorev1.ReplicationController
	dc *oappsv1.DeploymentConfig
}

// WaitForResources polls to get the current status of all deployments and stateful sets
// until they are ready or a timeout is reached
func (c *Client) WaitForResources(timeout time.Duration, resources []*manifest.MappingResult, flags common.WaitFlags) error {
	return wait.Poll(5*time.Second, timeout, func() (bool, error) {
		statefulSets := []appsv1.StatefulSet{}
		deployments := []deployment{}
		deploymentConfigs := []deploymentConfig{}
		for _, r := range resources {
			switch r.Metadata.Kind {
			case "ConfigMap":
			case "Service":
			case "ReplicationController":
			case "Pod":
			case "Deployment":
				if flags.WaitForDeployments {
					currentDeployment, err := c.kubeclientset.AppsV1().Deployments(r.Metadata.ObjectMeta.Namespace).Get(context.TODO(), r.Metadata.ObjectMeta.Name, kmetav1.GetOptions{})
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
				}
			case "DeploymentConfig":
				if flags.WaitForDeploymentConfigs {
					currentDC, err := c.occlientset.AppsV1().DeploymentConfigs(r.Metadata.ObjectMeta.Namespace).Get(context.TODO(), r.Metadata.ObjectMeta.Name, kmetav1.GetOptions{})
					if err != nil {
						return false, err
					}
					// Find RC associated with deploymentConfig
					newRC, err := c.getNewReplicationController(currentDC)
					if err != nil || newRC == nil {
						return false, err
					}
					newDC := deploymentConfig{newRC, currentDC}
					deploymentConfigs = append(deploymentConfigs, newDC)
				}
			case "StatefulSet":
				if flags.WaitForStatefulSets {
					sf, err := c.kubeclientset.AppsV1().StatefulSets(r.Metadata.ObjectMeta.Namespace).Get(context.TODO(), r.Metadata.ObjectMeta.Name, kmetav1.GetOptions{})
					if err != nil {
						return false, err
					}
					statefulSets = append(statefulSets, *sf)
				}
			}
		}

		// evaluate all the conditions first
		statefulSetsReady := c.statefulSetsReady(statefulSets)
		deploymentsReady := c.deploymentsReady(deployments)
		deploymentConfigsReady := c.deploymentConfigsReady(deploymentConfigs)

		isReady := statefulSetsReady && deploymentsReady && deploymentConfigsReady
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

func (c *Client) getNewReplicationController(deploymentConfig *oappsv1.DeploymentConfig) (*kcorev1.ReplicationController, error) {
	rcList, err := c.listReplicationControllers(deploymentConfig, c.rcListFromClient())
	if err != nil {
		return nil, err
	}
	return findNewReplicationController(deploymentConfig, rcList), nil
}

// RsListFunc returns the ReplicaSet from the ReplicaSet namespace and the List metav1.ListOptions.
type RsListFunc func(string, kmetav1.ListOptions) ([]*appsv1.ReplicaSet, error)

type RcListFunc func(string, kmetav1.ListOptions) ([]*kcorev1.ReplicationController, error)

// ListReplicaSets returns a slice of RSes the given deployment targets.
// Note that this does NOT attempt to reconcile ControllerRef (adopt/orphan),
// because only the controller itself should do that.
// However, it does filter out anything whose ControllerRef doesn't match.
func (c *Client) listReplicaSets(deployment *appsv1.Deployment, getRSList RsListFunc) ([]*appsv1.ReplicaSet, error) {
	// TODO: Right now we list replica sets by their labels. We should list them by selector, i.e. the replica set's selector
	//       should be a superset of the deployment's selector, see https://github.com/kubernetes/kubernetes/issues/19830.
	namespace := deployment.Namespace
	selector, err := kmetav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}
	options := kmetav1.ListOptions{LabelSelector: selector.String()}
	all, err := getRSList(namespace, options)
	if err != nil {
		return nil, err
	}
	// Only include those whose ControllerRef matches the Deployment.
	owned := make([]*appsv1.ReplicaSet, 0, len(all))
	for _, rs := range all {
		if kmetav1.IsControlledBy(rs, deployment) {
			owned = append(owned, rs)
		}
	}
	return owned, nil
}

func (c *Client) listReplicationControllers(deploymentConfig *oappsv1.DeploymentConfig, getRCList RcListFunc) ([]*kcorev1.ReplicationController, error) {
	// TODO: Right now we list reaplication controllers by their labels. We should list them by selector, i.e. the replica set's selector
	//       should be a superset of the deployment's selector, see https://github.com/kubernetes/kubernetes/issues/19830.
	namespace := deploymentConfig.Namespace
	selector := deploymentConfig.Spec.Selector
	options := kmetav1.ListOptions{LabelSelector: labels.Set(selector).String()}
	all, err := getRCList(namespace, options)
	if err != nil {
		return nil, err
	}
	// Only include those whose ControllerRef matches the Deployment.
	owned := make([]*kcorev1.ReplicationController, 0, len(all))
	for _, rc := range all {
		if kmetav1.IsControlledBy(rc, deploymentConfig) {
			owned = append(owned, rc)
		}
	}
	return owned, nil
}

// RsListFromClient returns an rsListFunc that wraps the given client.
func (c *Client) rsListFromClient() RsListFunc {
	return func(namespace string, options kmetav1.ListOptions) ([]*appsv1.ReplicaSet, error) {
		rsList, err := c.kubeclientset.AppsV1().ReplicaSets(namespace).List(context.TODO(), options)
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

func (c *Client) rcListFromClient() RcListFunc {
	return func(namespace string, options kmetav1.ListOptions) ([]*kcorev1.ReplicationController, error) {

		rcList, err := c.kubeclientset.CoreV1().ReplicationControllers(namespace).List(context.TODO(), options)
		if err != nil {
			return nil, err
		}
		var ret []*kcorev1.ReplicationController
		for i := range rcList.Items {
			ret = append(ret, &rcList.Items[i])
		}
		return ret, err
	}
}

// EqualIgnoreHash returns true if two given podTemplateSpec are equal, ignoring the diff in value of Labels[pod-template-hash]
// We ignore pod-template-hash because:
// 1. The hash result would be different upon podTemplateSpec API changes
//    (e.g. the addition of a new field will cause the hash code to change)
// 2. The deployment template won't have hash labels
func EqualIgnoreHash(template1, template2 *kcorev1.PodTemplateSpec) bool {
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

type ReplicationControllersByCreationTimestamp []*kcorev1.ReplicationController

func (o ReplicationControllersByCreationTimestamp) Len() int      { return len(o) }
func (o ReplicationControllersByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o ReplicationControllersByCreationTimestamp) Less(i, j int) bool {
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

func findNewReplicationController(deploymentConfig *oappsv1.DeploymentConfig, rcList []*kcorev1.ReplicationController) *kcorev1.ReplicationController {
	sort.Sort(sort.Reverse(ReplicationControllersByCreationTimestamp(rcList)))
	return rcList[0]
}

func (c *Client) statefulSetsReady(statefulsets []appsv1.StatefulSet) bool {
	for _, sf := range statefulsets {
		if sf.Status.UpdateRevision != sf.Status.CurrentRevision || sf.Status.ReadyReplicas != *sf.Spec.Replicas {
			fmt.Fprintf(c.out, "StatefulSet is not ready: %s/%s\n", sf.GetNamespace(), sf.GetName())
			return false
		}
	}
	return true
}

func (c *Client) deploymentsReady(deployments []deployment) bool {
	result := true
	for _, it := range deployments {
		if it.rs.Status.ReadyReplicas != *it.d.Spec.Replicas {
			fmt.Fprintf(c.out, "Deployment[%s] is not ready (%d/%d)\n", it.d.GetName(), it.rs.Status.ReadyReplicas, *it.d.Spec.Replicas)
			result = false
		} else {
			fmt.Fprintf(c.out, "Deployment[%s] is ready (%d/%d)\n", it.d.GetName(), it.rs.Status.ReadyReplicas, *it.d.Spec.Replicas)
		}
	}
	return result
}

func (c *Client) deploymentConfigsReady(deploymentConfigs []deploymentConfig) bool {
	result := true
	for _, it := range deploymentConfigs {
		if it.rc.Status.ReadyReplicas != it.dc.Spec.Replicas {
			fmt.Fprintf(c.out, "DeploymentConfig[name: %s, rc: %s] is not ready (%d/%d)\n", it.dc.GetName(), it.rc.GetName(), it.rc.Status.ReadyReplicas, it.dc.Spec.Replicas)
			result = false
		} else {
			fmt.Fprintf(c.out, "DeploymentConfig[name: %s, rc: %s] is ready (%d/%d)\n", it.dc.GetName(), it.rc.GetName(), it.rc.Status.ReadyReplicas, it.dc.Spec.Replicas)
		}
	}
	return result
}
