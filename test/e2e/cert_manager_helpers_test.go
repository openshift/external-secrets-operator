//go:build e2e
// +build e2e

/*
Copyright 2026.
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

package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"

	"github.com/openshift/external-secrets-operator/test/utils"
)

const (
	certManagerOperatorNamespace = "cert-manager-operator"
	certManagerOperandNamespace  = "cert-manager"
	certManagerOperatorManifest  = "testdata/cert-manager/operator.yaml"

	certManagerOperatorPodPrefix   = "cert-manager-operator-controller-manager-"
	certManagerCAInjectorPodPrefix = "cert-manager-cainjector-"
	certManagerWebhookPodPrefix    = "cert-manager-webhook-"

	certManagerInstallTimeout = 10 * time.Minute
)

var certificateGVR = schema.GroupVersionResource{
	Group:    "cert-manager.io",
	Version:  "v1",
	Resource: "certificates",
}

// ensureCertManagerOperatorReady installs the Red Hat cert-manager Operator via OLM if needed,
// then waits until the operator, operand pods, and Certificate API are ready.
// Idempotent: skips install when the Certificate API is already usable and operands are Ready.
func ensureCertManagerOperatorReady(ctx context.Context, clientset *kubernetes.Clientset, dynamicClient dynamic.Interface) error {
	By("Ensuring Red Hat cert-manager Operator is ready")

	if err := certManagerCertificateCRDInstalled(ctx, dynamicClient); err == nil {
		if err := waitForCertManagerOperandPods(ctx, clientset, 2*time.Minute); err == nil {
			By("cert-manager Certificate API and operand pods already ready")
			return nil
		}
	}

	By("Applying cert-manager Operator OLM manifests (Namespace, OperatorGroup, Subscription)")
	if err := utils.ApplyManifestFromReader(ctx, dynamicClient, testassets.ReadFile, certManagerOperatorManifest); err != nil {
		return fmt.Errorf("apply cert-manager operator manifests: %w", err)
	}

	By(fmt.Sprintf("Waiting for cert-manager operator pod in namespace %s", certManagerOperatorNamespace))
	if err := waitForReadyPodsByNamePrefixes(ctx, clientset, certManagerOperatorNamespace, []string{certManagerOperatorPodPrefix}, certManagerInstallTimeout); err != nil {
		return fmt.Errorf("wait for cert-manager operator pod: %w", err)
	}

	By(fmt.Sprintf("Waiting for cert-manager operand pods in namespace %s", certManagerOperandNamespace))
	if err := waitForCertManagerOperandPods(ctx, clientset, certManagerInstallTimeout); err != nil {
		return fmt.Errorf("wait for cert-manager operand pods: %w", err)
	}

	By("Waiting for cert-manager Certificate API")
	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		if err := certManagerCertificateCRDInstalled(ctx, dynamicClient); err != nil {
			return false, nil
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("cert-manager Certificate API not available after install: %w", err)
	}

	By("Red Hat cert-manager Operator is ready")
	return nil
}

// certManagerCertificateCRDInstalled returns nil when the cert-manager Certificate API is available.
func certManagerCertificateCRDInstalled(ctx context.Context, dynamicClient dynamic.Interface) error {
	_, err := dynamicClient.Resource(certificateGVR).Namespace("default").List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("cert-manager Certificate API unavailable: %w", err)
	}
	return nil
}

// waitForCertManagerOperandPods waits until controller, cainjector, and webhook pods are Ready.
func waitForCertManagerOperandPods(ctx context.Context, clientset kubernetes.Interface, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pods, err := clientset.CoreV1().Pods(certManagerOperandNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		var hasController, hasCAInjector, hasWebhook bool
		for i := range pods.Items {
			pod := &pods.Items[i]
			if pod.Status.Phase != corev1.PodRunning || !isPodReadyForCertManager(pod) {
				continue
			}
			name := pod.Name
			switch {
			case strings.HasPrefix(name, certManagerCAInjectorPodPrefix):
				hasCAInjector = true
			case strings.HasPrefix(name, certManagerWebhookPodPrefix):
				hasWebhook = true
			case strings.HasPrefix(name, "cert-manager-") &&
				!strings.HasPrefix(name, certManagerCAInjectorPodPrefix) &&
				!strings.HasPrefix(name, certManagerWebhookPodPrefix) &&
				!strings.Contains(name, "startupapicheck"):
				// Main controller Deployment pods: cert-manager-<replicaset>-<pod>
				hasController = true
			}
		}
		return hasController && hasCAInjector && hasWebhook, nil
	})
}

func waitForReadyPodsByNamePrefixes(ctx context.Context, clientset kubernetes.Interface, namespace string, prefixes []string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		matched := make(map[string]bool, len(prefixes))
		for _, prefix := range prefixes {
			matched[prefix] = false
		}
		for i := range pods.Items {
			pod := &pods.Items[i]
			if pod.Status.Phase != corev1.PodRunning || !isPodReadyForCertManager(pod) {
				continue
			}
			for _, prefix := range prefixes {
				if strings.HasPrefix(pod.Name, prefix) {
					matched[prefix] = true
				}
			}
		}
		for _, ok := range matched {
			if !ok {
				return false, nil
			}
		}
		return true, nil
	})
}

func isPodReadyForCertManager(pod *corev1.Pod) bool {
	ready := map[string]bool{
		"Ready":           false,
		"ContainersReady": false,
	}
	for _, cond := range pod.Status.Conditions {
		if _, ok := ready[string(cond.Type)]; ok && cond.Status == corev1.ConditionTrue {
			ready[string(cond.Type)] = true
		}
	}
	return ready["Ready"] && ready["ContainersReady"]
}
