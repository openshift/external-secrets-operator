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
	"testing"
	"time"

	. "github.com/onsi/gomega"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	externalsecrets "github.com/openshift/external-secrets-operator/pkg/controller/external_secrets"
	"github.com/openshift/external-secrets-operator/test/utils"
)

const (
	operatorDeploymentName       = common.ExternalSecretsOperatorCommonName + "-controller-manager"
	operatorManagerContainerName = "manager"
	// operatorCSVNamePrefix matches ClusterServiceVersion names like
	// openshift-external-secrets-operator.v1.2.0.
	operatorCSVNamePrefix = "openshift-external-secrets-operator."
	// operatorPackageName is the OLM package / Subscription.spec.name value.
	operatorPackageName = "openshift-external-secrets-operator"
)

// ensureExternalSecretsConfigReady creates the cluster ExternalSecretsConfig CR when missing
// and waits until Ready=True. Shared by suite Describes that may run before e2e_test BeforeAll.
func ensureExternalSecretsConfigReady(ctx context.Context) error {
	if suiteRuntimeClient == nil || suiteDynamicClient == nil || suiteClientset == nil {
		return fmt.Errorf("suite clients not initialized")
	}

	if err := waitForNamespaceTermination(ctx, suiteClientset, externalsecrets.OperandDefaultNamespace, 2*time.Minute); err != nil {
		return fmt.Errorf("waiting for operand namespace to finish terminating: %w", err)
	}

	esc := &operatorv1alpha1.ExternalSecretsConfig{}
	err := suiteRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		loader := utils.NewDynamicResourceLoader(ctx, &testing.T{})
		loader.CreateFromFile(testassets.ReadFile, externalSecretsFile, "")
	}

	return utils.WaitForExternalSecretsConfigReady(ctx, suiteDynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)
}

func waitForNamespaceTermination(ctx context.Context, clientset *kubernetes.Clientset, namespace string, timeout time.Duration) error {
	ns, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if ns.Status.Phase != "Terminating" {
		return nil
	}
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
}

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

// componentConfigsForESC returns component configs that apply to deployments present for the given ESC.
func componentConfigsForESC(esc *operatorv1alpha1.ExternalSecretsConfig, configs []operatorv1alpha1.ComponentConfig) []operatorv1alpha1.ComponentConfig {
	if utils.IsCertControllerExpected(esc) {
		return configs
	}
	filtered := make([]operatorv1alpha1.ComponentConfig, 0, len(configs))
	for _, cfg := range configs {
		if cfg.ComponentName != operatorv1alpha1.CertController {
			filtered = append(filtered, cfg)
		}
	}
	return filtered
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
// The second return value indicates whether the container was found.
func deploymentContainerHasArg(deployment *appsv1.Deployment, containerName, arg string) (bool, bool) {
	args, found := getDeploymentContainerArgs(deployment, containerName)
	if !found {
		return false, false
	}
	return slices.Contains(args, arg), true
}

// setOperatorManagerEnv sets or updates env vars on the operator manager container.
// Prefer updating Subscription.spec.config.env (OLM-supported) when a matching CSV
// and Subscription exist; otherwise update the Deployment directly.
// Works for any manager env (OPERAND_*_ARGS, OPERATOR_LOG_LEVEL, METRICS_*, etc.).
// OLM rolls a new manager pod after Subscription updates; the startup reconcile reads
// process env and applies operand args. This helper waits until that Ready pod has the
// desired env before returning.
func setOperatorManagerEnv(ctx context.Context, clientset kubernetes.Interface, c client.Client, envVars map[string]string) error {
	if len(envVars) == 0 {
		return nil
	}
	updatedViaSub, err := updateSubscriptionEnv(ctx, clientset, c, envVars, nil)
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
func unsetOperatorManagerEnv(ctx context.Context, clientset kubernetes.Interface, c client.Client, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	updatedViaSub, err := updateSubscriptionEnv(ctx, clientset, c, nil, keys)
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

// updateSubscriptionEnv merges env into Subscription.spec.config.env using the CSV to
// locate the Subscription namespace. Returns false when no OLM Subscription is found.
func updateSubscriptionEnv(ctx context.Context, clientset kubernetes.Interface, c client.Client, set map[string]string, unset []string) (bool, error) {
	csv, err := findOperatorCSV(ctx, clientset, c)
	if err != nil {
		return false, err
	}
	if csv == nil {
		return false, nil
	}

	subNamespace := csv.Annotations[olmv1alpha1.OperatorGroupNamespaceAnnotationKey]
	if subNamespace == "" {
		subNamespace = csv.Namespace
	}
	if subNamespace == "" {
		return false, fmt.Errorf("CSV %s has empty namespace and no %s annotation", csv.Name, olmv1alpha1.OperatorGroupNamespaceAnnotationKey)
	}

	sub, err := findOperatorSubscription(ctx, c, subNamespace)
	if err != nil {
		return false, err
	}
	if sub == nil {
		return false, nil
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &olmv1alpha1.Subscription{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: subNamespace, Name: sub.Name}, current); err != nil {
			return fmt.Errorf("get Subscription %s/%s: %w", subNamespace, sub.Name, err)
		}

		var existing []corev1.EnvVar
		if current.Spec.Config != nil {
			existing = current.Spec.Config.Env
		}
		merged := mergeEnvVars(existing, set, unset)
		if current.Spec.Config == nil {
			if len(merged) == 0 {
				return nil
			}
			current.Spec.Config = &olmv1alpha1.SubscriptionConfig{}
		}
		current.Spec.Config.Env = merged
		if isEmptySubscriptionConfig(current.Spec.Config) {
			current.Spec.Config = nil
		}

		return c.Update(ctx, current)
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// findOperatorCSV returns the installed openshift-external-secrets-operator CSV, or nil
// when the operator is not managed by OLM.
func findOperatorCSV(ctx context.Context, clientset kubernetes.Interface, c client.Client) (*olmv1alpha1.ClusterServiceVersion, error) {
	var list olmv1alpha1.ClusterServiceVersionList
	if err := c.List(ctx, &list, client.InNamespace(operatorNamespace)); err != nil {
		// NotFound / NoMatch: OLM CRDs missing or no CSVs — fall through to Deployment path.
		if !k8serrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return nil, fmt.Errorf("list CSVs in %s: %w", operatorNamespace, err)
		}
	}
	for i := range list.Items {
		if strings.HasPrefix(list.Items[i].Name, operatorCSVNamePrefix) {
			return list.Items[i].DeepCopy(), nil
		}
	}

	// Fallback: resolve CSV from the operator Deployment ownerReference.
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
		csv := &olmv1alpha1.ClusterServiceVersion{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: operatorNamespace, Name: ref.Name}, csv); err != nil {
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

// findOperatorSubscription returns the Subscription for the operator package in ns.
func findOperatorSubscription(ctx context.Context, c client.Client, ns string) (*olmv1alpha1.Subscription, error) {
	var list olmv1alpha1.SubscriptionList
	if err := c.List(ctx, &list, client.InNamespace(ns)); err != nil {
		return nil, fmt.Errorf("list Subscriptions in %s: %w", ns, err)
	}
	for i := range list.Items {
		item := &list.Items[i]
		if item.Spec.Package == operatorPackageName || strings.HasPrefix(item.Name, operatorPackageName) {
			return item.DeepCopy(), nil
		}
	}
	return nil, nil
}

func isEmptySubscriptionConfig(c *olmv1alpha1.SubscriptionConfig) bool {
	if c == nil {
		return true
	}
	return c.Selector == nil &&
		len(c.NodeSelector) == 0 &&
		len(c.Tolerations) == 0 &&
		c.Resources == nil &&
		len(c.EnvFrom) == 0 &&
		len(c.Env) == 0 &&
		len(c.Volumes) == 0 &&
		len(c.VolumeMounts) == 0 &&
		c.Affinity == nil &&
		len(c.Annotations) == 0
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
	// Sort newly appended names so repeated merges produce a stable Env order
	// (map iteration order is nondeterministic and can trigger an extra OLM rollout).
	toAdd := make([]string, 0, len(set))
	for name := range set {
		if !seen[name] {
			toAdd = append(toAdd, name)
		}
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

		// Deployment env can update before the rolled pod is the Ready one; os.Getenv in the
		// manager only sees the running pod's env, so wait for that too.
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
