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
func WaitForESOResourceReady(
	ctx context.Context,
	client dynamic.Interface,
	gvr schema.GroupVersionResource,
	namespace, name string,
	timeout time.Duration,
) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		u, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // retry
		}

		conds, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
		if err != nil || !found {
			return false, nil // retry
		}

		for _, c := range conds {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			t := cond["type"]
			s := cond["status"]
			msg := cond["message"]

			if t == "Ready" {
				if s == "True" {
					return true, nil
				} else {
					fmt.Printf("resource %s/%s not ready: %v\n", namespace, name, msg)
				}
			}
		}
		return false, nil
	})
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

func DeleteVaultSecret(ctx context.Context, k8sClient *kubernetes.Clientset, secretName, region string) error {
	return nil
}
