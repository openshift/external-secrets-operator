//go:build e2e
// +build e2e

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

package utils

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/aws/aws-sdk-go/aws"
	awscred "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

const (
	awsCredSecretName             = "aws-creds"
	awsCredNamespace              = "kube-system"
	awsCredAccessKeySecretKeyName = "aws_secret_access_key"
	awsCredKeyIdSecretKeyName     = "aws_access_key_id"
)

type AssetFunc func(string) ([]byte, error)

// VerifyPodsReadyByPrefix checks if all pods matching the given prefixes are Ready and ContainersReady.
func VerifyPodsReadyByPrefix(ctx context.Context, clientset kubernetes.Interface, namespace string, prefixes []string) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		matched := map[string]*corev1.Pod{}
		for _, pod := range podList.Items {
			for _, prefix := range prefixes {
				if strings.HasPrefix(pod.Name, prefix) {
					matched[pod.Name] = &pod
				}
			}
		}

		if len(matched) != len(prefixes) {
			return false, nil
		}

		for _, pod := range matched {
			if pod.Status.Phase != corev1.PodRunning || !isPodReady(pod) {
				return false, nil
			}
		}

		return true, nil
	})
}

// isPodReady checks PodReady and ContainersReady conditions.
func isPodReady(pod *corev1.Pod) bool {
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

// WaitForESOResourceReady checks if a custom ESO resource (like SecretStore/PushSecret) is Ready=True
// and not Degraded. Returns early with error if Degraded condition is true.
func WaitForESOResourceReady(
	ctx context.Context,
	client dynamic.Interface,
	gvr schema.GroupVersionResource,
	namespace, name string,
	timeout time.Duration,
) error {
	var lastConditions []map[string]interface{}

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		u, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // retry
		}

		conds, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
		if err != nil || !found {
			return false, nil // retry
		}

		// Store conditions for timeout error message
		lastConditions = make([]map[string]interface{}, 0, len(conds))

		var readyCondition, degradedCondition map[string]interface{}
		for _, c := range conds {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			lastConditions = append(lastConditions, cond)

			t := cond["type"]
			if t == "Ready" {
				readyCondition = cond
			}
			if t == "Degraded" {
				degradedCondition = cond
			}
		}

		// Check for Degraded condition first - fail fast if degraded
		if degradedCondition != nil && degradedCondition["status"] == "True" {
			return false, fmt.Errorf("resource %s/%s is degraded: %v", namespace, name, degradedCondition["message"])
		}

		// Check Ready condition
		if readyCondition != nil {
			if readyCondition["status"] == "True" {
				return true, nil
			}
			fmt.Printf("resource %s/%s not ready: %v\n", namespace, name, readyCondition["message"])
		}

		return false, nil
	})

	// Provide detailed error message on timeout
	if err != nil && wait.Interrupted(err) {
		conditionSummary := "no conditions found"
		if len(lastConditions) > 0 {
			var parts []string
			for _, c := range lastConditions {
				parts = append(parts, fmt.Sprintf("%s=%s", c["type"], c["status"]))
			}
			conditionSummary = strings.Join(parts, ", ")
		}
		return fmt.Errorf("timeout waiting for %s %s/%s to be ready: %s", gvr.Resource, namespace, name, conditionSummary)
	}

	return err
}

// externalSecretsConfigGVR is the GroupVersionResource for ExternalSecretsConfig
var externalSecretsConfigGVR = schema.GroupVersionResource{
	Group:    "operator.openshift.io",
	Version:  "v1alpha1",
	Resource: "externalsecretsconfigs",
}

// WaitForExternalSecretsConfigReady checks if the ExternalSecretsConfig CR has Ready=True and Degraded=False.
// This verifies that the operator has successfully reconciled the configuration.
// Returns early with error if Degraded condition is true.
func WaitForExternalSecretsConfigReady(
	ctx context.Context,
	client dynamic.Interface,
	name string,
	timeout time.Duration,
) error {
	var lastReadyCondition, lastDegradedCondition map[string]interface{}

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		u, err := client.Resource(externalSecretsConfigGVR).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // retry on not found or other errors
		}

		conds, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
		if err != nil {
			return false, fmt.Errorf("failed to extract conditions from ExternalSecretsConfig: %w", err)
		}
		if !found {
			return false, nil // conditions not yet set, retry
		}

		for _, c := range conds {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _ := cond["type"].(string)
			switch condType {
			case "Ready":
				lastReadyCondition = cond
			case "Degraded":
				lastDegradedCondition = cond
			}
		}

		// Check for Degraded condition first - fail fast if degraded
		if lastDegradedCondition != nil && lastDegradedCondition["status"] == "True" {
			return false, fmt.Errorf("ExternalSecretsConfig %s is degraded: %v", name, lastDegradedCondition["message"])
		}

		// Check Ready condition
		if lastReadyCondition == nil {
			return false, nil // Ready condition not yet set, retry
		}

		return lastReadyCondition["status"] == "True", nil
	})

	// Provide detailed error message on timeout
	if err != nil && wait.Interrupted(err) {
		readyStatus := "not set"
		degradedStatus := "not set"
		if lastReadyCondition != nil {
			readyStatus = fmt.Sprintf("%v (reason: %v, message: %v)",
				lastReadyCondition["status"], lastReadyCondition["reason"], lastReadyCondition["message"])
		}
		if lastDegradedCondition != nil {
			degradedStatus = fmt.Sprintf("%v (reason: %v, message: %v)",
				lastDegradedCondition["status"], lastDegradedCondition["reason"], lastDegradedCondition["message"])
		}
		return fmt.Errorf("timeout waiting for ExternalSecretsConfig %s to be ready: Ready=%s, Degraded=%s",
			name, readyStatus, degradedStatus)
	}

	return err
}

func fetchAWSCreds(ctx context.Context, k8sClient *kubernetes.Clientset) (string, string, error) {
	cred, err := k8sClient.CoreV1().Secrets(awsCredNamespace).Get(ctx, awsCredSecretName, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	id := string(cred.Data[awsCredKeyIdSecretKeyName])
	key := string(cred.Data[awsCredAccessKeySecretKeyName])
	return id, key, nil
}

func DeleteAWSSecret(ctx context.Context, k8sClient *kubernetes.Clientset, secretName, region string) error {
	id, key, err := fetchAWSCreds(ctx, k8sClient)
	if err != nil {
		return err
	}

	sess, err := session.NewSession(&aws.Config{
		Credentials: awscred.NewCredentials(&awscred.StaticProvider{Value: awscred.Value{
			AccessKeyID:     id,
			SecretAccessKey: key,
		}}),
		Region: aws.String(region),
	})
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %w", err)
	}

	svc := secretsmanager.New(sess)
	_, err = svc.DeleteSecret(&secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(secretName),
		ForceDeleteWithoutRecovery: aws.Bool(true), // permanently delete without 7-day wait
	})
	if err != nil {
		return fmt.Errorf("failed to delete AWS secret: %w", err)
	}
	return nil
}

func ReadExpectedSecretValue(assetName string) ([]byte, error) {
	expectedSecretValue, err := os.ReadFile(assetName)
	return expectedSecretValue, err
}

// GetRandomString to create random string
func GetRandomString(strLen int) string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, strLen)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

func ReplacePatternInAsset(replacePatternString ...string) AssetFunc {
	return func(assetName string) ([]byte, error) {
		fileContent, err := os.ReadFile(assetName)
		if err != nil {
			return nil, err
		}

		replacer := strings.NewReplacer(replacePatternString...)
		replacedFileContent := replacer.Replace(string(fileContent))
		return []byte(replacedFileContent), nil
	}
}
