//go:build e2e
// +build e2e

package e2e

/*
Copyright 2025.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

const (
	operatorDeploymentName       = common.ExternalSecretsOperatorCommonName + "-controller-manager"
	operatorManagerContainerName = "manager"
	// operatorCSVNamePrefix matches ClusterServiceVersion names like
	// openshift-external-secrets-operator.v1.1.0.
	operatorCSVNamePrefix = "openshift-external-secrets-operator."
	// operatorPackageName is the OLM package / Subscription.spec.name value.
	operatorPackageName            = "openshift-external-secrets-operator"
	olmOperatorNamespaceAnnotation = "olm.operatorNamespace"
)

var (
	csvGVR = schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "clusterserviceversions",
	}
	subscriptionGVR = schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "subscriptions",
	}
	externalSecretsConfigGVR = schema.GroupVersionResource{
		Group:    "operator.openshift.io",
		Version:  "v1alpha1",
		Resource: "externalsecretsconfigs",
	}
)

// resourceType defines a Kubernetes resource type to verify annotations on
type resourceType struct {
	name         string
	listFunc     func(ctx context.Context, clientset *kubernetes.Clientset, namespace string, g Gomega) ([]metav1.Object, error)
	checkPodSpec bool
}

// getResourceTypesToVerify returns the list of resource types that should have annotations verified
func getResourceTypesToVerify() []resourceType {
	listOnlyManagedResources := metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=external-secrets-operator",
	}

	return []resourceType{
		{
			name: "Deployment",
			listFunc: func(ctx context.Context, clientset *kubernetes.Clientset, namespace string, g Gomega) ([]metav1.Object, error) {
				deployments, err := clientset.AppsV1().Deployments(namespace).List(ctx, listOnlyManagedResources)
				if err != nil {
					return nil, err
				}
				objects := make([]metav1.Object, 0, len(deployments.Items))
				for i := range deployments.Items {
					objects = append(objects, &deployments.Items[i])
				}
				return objects, nil
			},
			checkPodSpec: true,
		},
		{
			name: "Service",
			listFunc: func(ctx context.Context, clientset *kubernetes.Clientset, namespace string, g Gomega) ([]metav1.Object, error) {
				services, err := clientset.CoreV1().Services(namespace).List(ctx, listOnlyManagedResources)
				if err != nil {
					return nil, err
				}
				objects := make([]metav1.Object, 0, len(services.Items))
				for i := range services.Items {
					objects = append(objects, &services.Items[i])
				}
				return objects, nil
			},
		},
		{
			name: "ServiceAccount",
			listFunc: func(ctx context.Context, clientset *kubernetes.Clientset, namespace string, g Gomega) ([]metav1.Object, error) {
				serviceAccounts, err := clientset.CoreV1().ServiceAccounts(namespace).List(ctx, listOnlyManagedResources)
				if err != nil {
					return nil, err
				}
				objects := make([]metav1.Object, 0, len(serviceAccounts.Items))
				for i := range serviceAccounts.Items {
					objects = append(objects, &serviceAccounts.Items[i])
				}
				return objects, nil
			},
		},
		{
			name: "ConfigMap",
			listFunc: func(ctx context.Context, clientset *kubernetes.Clientset, namespace string, g Gomega) ([]metav1.Object, error) {
				configMaps, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, listOnlyManagedResources)
				if err != nil {
					return nil, err
				}
				objects := make([]metav1.Object, 0, len(configMaps.Items))
				for i := range configMaps.Items {
					objects = append(objects, &configMaps.Items[i])
				}
				return objects, nil
			},
		},
		{
			name: "NetworkPolicy",
			listFunc: func(ctx context.Context, clientset *kubernetes.Clientset, namespace string, g Gomega) ([]metav1.Object, error) {
				networkPolicies, err := clientset.NetworkingV1().NetworkPolicies(namespace).List(ctx, listOnlyManagedResources)
				if err != nil {
					return nil, err
				}
				objects := make([]metav1.Object, 0, len(networkPolicies.Items))
				for i := range networkPolicies.Items {
					objects = append(objects, &networkPolicies.Items[i])
				}
				return objects, nil
			},
		},
		{
			name: "Role",
			listFunc: func(ctx context.Context, clientset *kubernetes.Clientset, namespace string, g Gomega) ([]metav1.Object, error) {
				roles, err := clientset.RbacV1().Roles(namespace).List(ctx, listOnlyManagedResources)
				if err != nil {
					return nil, err
				}
				objects := make([]metav1.Object, 0, len(roles.Items))
				for i := range roles.Items {
					objects = append(objects, &roles.Items[i])
				}
				return objects, nil
			},
		},
		{
			name: "RoleBinding",
			listFunc: func(ctx context.Context, clientset *kubernetes.Clientset, namespace string, g Gomega) ([]metav1.Object, error) {
				roleBindings, err := clientset.RbacV1().RoleBindings(namespace).List(ctx, listOnlyManagedResources)
				if err != nil {
					return nil, err
				}
				objects := make([]metav1.Object, 0, len(roleBindings.Items))
				for i := range roleBindings.Items {
					objects = append(objects, &roleBindings.Items[i])
				}
				return objects, nil
			},
		},
		{
			name: "Secret",
			listFunc: func(ctx context.Context, clientset *kubernetes.Clientset, namespace string, g Gomega) ([]metav1.Object, error) {
				secrets, err := clientset.CoreV1().Secrets(namespace).List(ctx, listOnlyManagedResources)
				if err != nil {
					return nil, err
				}
				objects := make([]metav1.Object, 0, len(secrets.Items))
				for i := range secrets.Items {
					objects = append(objects, &secrets.Items[i])
				}
				return objects, nil
			},
		},
	}
}

// asDeployment safely casts a metav1.Object to an appsv1.Deployment
func asDeployment(obj metav1.Object) *appsv1.Deployment {
	return obj.(*appsv1.Deployment)
}

// getDeploymentContainerArgs returns container args for the named container in a deployment.
func getDeploymentContainerArgs(deployment *appsv1.Deployment, containerName string) ([]string, bool) {
	if deployment == nil {
		return nil, false
	}
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			return container.Args, true
		}
	}
	return nil, false
}

// deploymentContainerHasArg reports whether the named container has the given arg.
func deploymentContainerHasArg(deployment *appsv1.Deployment, containerName, arg string) (bool, bool) {
	args, found := getDeploymentContainerArgs(deployment, containerName)
	if !found {
		return false, false
	}
	return slices.Contains(args, arg), true
}

// setOperatorManagerEnv sets or updates env vars on the operator manager container.
// Prefer updating Subscription.spec.config.env when a matching CSV and Subscription
// exist; otherwise update the Deployment directly.
func setOperatorManagerEnv(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, envVars map[string]string) error {
	if len(envVars) == 0 {
		return nil
	}
	updatedViaSub, err := updateSubscriptionEnv(ctx, clientset, dynamicClient, envVars, nil)
	if err != nil {
		return err
	}
	if !updatedViaSub {
		if err := updateOperatorDeploymentEnv(ctx, clientset, envVars, nil); err != nil {
			return err
		}
	}
	waitForOperatorManagerEnv(ctx, clientset, envVars, nil)
	return nil
}

// unsetOperatorManagerEnv removes env vars from the operator manager container.
func unsetOperatorManagerEnv(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	updatedViaSub, err := updateSubscriptionEnv(ctx, clientset, dynamicClient, nil, keys)
	if err != nil {
		return err
	}
	if !updatedViaSub {
		if err := updateOperatorDeploymentEnv(ctx, clientset, nil, keys); err != nil {
			return err
		}
	}
	waitForOperatorManagerEnv(ctx, clientset, nil, keys)
	return nil
}

func updateSubscriptionEnv(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, set map[string]string, unset []string) (bool, error) {
	csv, err := findOperatorCSV(ctx, clientset, dynamicClient)
	if err != nil {
		return false, err
	}
	if csv == nil {
		return false, nil
	}

	subNamespace := csv.GetAnnotations()[olmOperatorNamespaceAnnotation]
	if subNamespace == "" {
		subNamespace = csv.GetNamespace()
	}
	if subNamespace == "" {
		return false, fmt.Errorf("CSV %s has empty namespace and no %s annotation", csv.GetName(), olmOperatorNamespaceAnnotation)
	}

	sub, err := findOperatorSubscription(ctx, dynamicClient, subNamespace)
	if err != nil {
		return false, err
	}
	if sub == nil {
		return false, nil
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := dynamicClient.Resource(subscriptionGVR).Namespace(subNamespace).Get(ctx, sub.GetName(), metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get Subscription %s/%s: %w", subNamespace, sub.GetName(), err)
		}

		config, _, _ := unstructured.NestedMap(current.Object, "spec", "config")
		if config == nil {
			config = map[string]interface{}{}
		}
		rawEnv, _, _ := unstructured.NestedSlice(config, "env")
		merged := mergeUnstructuredEnv(rawEnv, set, unset)
		if len(merged) == 0 {
			delete(config, "env")
		} else {
			config["env"] = merged
		}
		if len(config) == 0 {
			unstructured.RemoveNestedField(current.Object, "spec", "config")
		} else if err := unstructured.SetNestedMap(current.Object, config, "spec", "config"); err != nil {
			return err
		}

		_, err = dynamicClient.Resource(subscriptionGVR).Namespace(subNamespace).Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func findOperatorCSV(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface) (*unstructured.Unstructured, error) {
	list, err := dynamicClient.Resource(csvGVR).Namespace(operatorNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return nil, fmt.Errorf("list CSVs in %s: %w", operatorNamespace, err)
		}
	}
	if list != nil {
		for i := range list.Items {
			if strings.HasPrefix(list.Items[i].GetName(), operatorCSVNamePrefix) {
				return list.Items[i].DeepCopy(), nil
			}
		}
	}

	dep, err := clientset.AppsV1().Deployments(operatorNamespace).Get(ctx, operatorDeploymentName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get operator deployment: %w", err)
	}
	for _, ref := range dep.OwnerReferences {
		if ref.Kind != "ClusterServiceVersion" || ref.Name == "" {
			continue
		}
		csv, err := dynamicClient.Resource(csvGVR).Namespace(operatorNamespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			if meta.IsNoMatchError(err) {
				return nil, nil
			}
			if k8serrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("get CSV %s/%s: %w", operatorNamespace, ref.Name, err)
		}
		return csv, nil
	}
	return nil, nil
}

func findOperatorSubscription(ctx context.Context, dynamicClient dynamic.Interface, ns string) (*unstructured.Unstructured, error) {
	list, err := dynamicClient.Resource(subscriptionGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		if meta.IsNoMatchError(err) || k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list Subscriptions in %s: %w", ns, err)
	}
	for i := range list.Items {
		item := &list.Items[i]
		pkg, _, _ := unstructured.NestedString(item.Object, "spec", "name")
		if pkg == operatorPackageName || strings.HasPrefix(item.GetName(), operatorPackageName) {
			return item.DeepCopy(), nil
		}
	}
	return nil, nil
}

func updateOperatorDeploymentEnv(ctx context.Context, clientset kubernetes.Interface, set map[string]string, unset []string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		dep, err := clientset.AppsV1().Deployments(operatorNamespace).Get(ctx, operatorDeploymentName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		idx := managerContainerIndex(dep.Spec.Template.Spec.Containers)
		if idx < 0 {
			return fmt.Errorf("manager container not found in operator deployment")
		}
		dep.Spec.Template.Spec.Containers[idx].Env = mergeEnvVars(dep.Spec.Template.Spec.Containers[idx].Env, set, unset)
		_, err = clientset.AppsV1().Deployments(operatorNamespace).Update(ctx, dep, metav1.UpdateOptions{})
		return err
	})
}

func mergeUnstructuredEnv(raw []interface{}, set map[string]string, unset []string) []interface{} {
	remove := make(map[string]struct{}, len(unset))
	for _, k := range unset {
		remove[k] = struct{}{}
	}
	seen := make(map[string]bool)
	out := make([]interface{}, 0, len(raw)+len(set))
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _, _ := unstructured.NestedString(m, "name")
		if name == "" {
			continue
		}
		if _, drop := remove[name]; drop {
			continue
		}
		if val, ok := set[name]; ok {
			m["value"] = val
			delete(m, "valueFrom")
		}
		out = append(out, m)
		seen[name] = true
	}
	toAdd := make([]string, 0, len(set))
	for name := range set {
		if seen[name] {
			continue
		}
		toAdd = append(toAdd, name)
	}
	slices.Sort(toAdd)
	for _, name := range toAdd {
		out = append(out, map[string]interface{}{"name": name, "value": set[name]})
	}
	return out
}

func managerContainerIndex(containers []corev1.Container) int {
	for i, c := range containers {
		if c.Name == operatorManagerContainerName {
			return i
		}
	}
	return -1
}

func mergeEnvVars(existing []corev1.EnvVar, set map[string]string, unset []string) []corev1.EnvVar {
	remove := make(map[string]struct{}, len(unset))
	for _, k := range unset {
		remove[k] = struct{}{}
	}
	out := make([]corev1.EnvVar, 0, len(existing)+len(set))
	seen := make(map[string]bool, len(existing))
	for _, env := range existing {
		if _, drop := remove[env.Name]; drop {
			continue
		}
		if val, ok := set[env.Name]; ok {
			env.Value = val
			env.ValueFrom = nil
		}
		out = append(out, env)
		seen[env.Name] = true
	}
	toAdd := make([]string, 0, len(set))
	for name := range set {
		if seen[name] {
			continue
		}
		toAdd = append(toAdd, name)
	}
	slices.Sort(toAdd)
	for _, name := range toAdd {
		out = append(out, corev1.EnvVar{Name: name, Value: set[name]})
	}
	return out
}

func waitForOperatorManagerEnv(ctx context.Context, clientset kubernetes.Interface, want map[string]string, unset []string) {
	Eventually(func(g Gomega) {
		dep, err := clientset.AppsV1().Deployments(operatorNamespace).Get(ctx, operatorDeploymentName, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		idx := managerContainerIndex(dep.Spec.Template.Spec.Containers)
		g.Expect(idx).To(BeNumerically(">=", 0), "manager container should exist")
		assertEnvMap(g, envSliceToMap(dep.Spec.Template.Spec.Containers[idx].Env), want, unset, "operator Deployment")

		pods, err := clientset.CoreV1().Pods(operatorNamespace).List(ctx, metav1.ListOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		var readyPod *corev1.Pod
		for i := range pods.Items {
			pod := &pods.Items[i]
			if pod.DeletionTimestamp != nil || !strings.HasPrefix(pod.Name, operatorPodPrefix) {
				continue
			}
			if pod.Status.Phase == corev1.PodRunning && isOperatorPodReady(pod) {
				readyPod = pod
				break
			}
		}
		g.Expect(readyPod).NotTo(BeNil(), "expected a Ready non-terminating operator manager pod")
		cidx := managerContainerIndex(readyPod.Spec.Containers)
		g.Expect(cidx).To(BeNumerically(">=", 0), "manager container should exist on Ready pod %s", readyPod.Name)
		assertEnvMap(g, envSliceToMap(readyPod.Spec.Containers[cidx].Env), want, unset, "operator pod "+readyPod.Name)
	}, 3*time.Minute, 5*time.Second).Should(Succeed())
}

func envSliceToMap(env []corev1.EnvVar) map[string]string {
	out := make(map[string]string, len(env))
	for _, e := range env {
		out[e.Name] = e.Value
	}
	return out
}

func assertEnvMap(g Gomega, envMap map[string]string, want map[string]string, unset []string, where string) {
	for name, val := range want {
		g.Expect(envMap).To(HaveKeyWithValue(name, val), "%s should have env %s=%s", where, name, val)
	}
	for _, name := range unset {
		g.Expect(envMap).NotTo(HaveKey(name), "%s should not have env %s", where, name)
	}
}

func isOperatorPodReady(pod *corev1.Pod) bool {
	ready, containersReady := false, false
	for _, cond := range pod.Status.Conditions {
		if cond.Status != corev1.ConditionTrue {
			continue
		}
		switch cond.Type {
		case corev1.PodReady:
			ready = true
		case corev1.ContainersReady:
			containersReady = true
		}
	}
	return ready && containersReady
}

func isExternalSecretsConfigDegraded(ctx context.Context) bool {
	u, err := suiteDynamicClient.Resource(externalSecretsConfigGVR).Get(ctx, common.ExternalSecretsConfigObjectName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	conds, found, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	if !found {
		return false
	}
	for _, c := range conds {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == operatorv1alpha1.Degraded && cond["status"] == string(metav1.ConditionTrue) {
			return true
		}
	}
	return false
}

func externalSecretsConfigDegradedMessage(ctx context.Context) string {
	u, err := suiteDynamicClient.Resource(externalSecretsConfigGVR).Get(ctx, common.ExternalSecretsConfigObjectName, metav1.GetOptions{})
	if err != nil {
		return ""
	}
	conds, found, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	if !found {
		return ""
	}
	for _, c := range conds {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == operatorv1alpha1.Degraded && cond["status"] == string(metav1.ConditionTrue) {
			msg, _, _ := unstructured.NestedString(cond, "message")
			return msg
		}
	}
	return ""
}
