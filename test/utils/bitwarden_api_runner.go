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
	"encoding/json"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	bitwardenAPITestRunnerImage = "docker.io/curlimages/curl:latest"
	bitwardenCredMountPath      = "/etc/bitwarden-cred"
)

// BitwardenCredMountPath returns the path inside API test pods where the Bitwarden cred secret is mounted.
func BitwardenCredMountPath() string {
	return bitwardenCredMountPath
}

// RunBitwardenAPIJob runs a one-off Job in the given namespace with the Bitwarden cred secret mounted.
// The job runs the given command (e.g. a shell script that curls the Bitwarden API and exits 0 on success).
// Returns the container exit code (0 = success), pod logs, and any error (e.g. timeout, job failed).
// Caller should use a unique jobName per test to avoid conflicts.
// Job spec is minimal (no security context); the platform (e.g. OpenShift SCC) mutates as needed.
// Pod label app.kubernetes.io/name=external-secrets matches the network policy so the Job can reach bitwarden-sdk-server.
func RunBitwardenAPIJob(ctx context.Context, client kubernetes.Interface, namespace, jobName string, command []string, timeout time.Duration) (exitCode int, logs string, err error) {
	backOffLimit := int32(0)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app.kubernetes.io/name": "external-secrets"},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:    "curl",
						Image:   bitwardenAPITestRunnerImage,
						Command: command,
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "bitwarden-cred",
							MountPath: bitwardenCredMountPath,
							ReadOnly:  true,
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "bitwarden-cred",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: BitwardenCredSecretName,
							},
						},
					}},
				},
			},
		},
	}
	_, err = client.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return -1, "", fmt.Errorf("create job %s: %w", jobName, err)
	}
	propagationBackground := metav1.DeletePropagationBackground
	defer func() {
		_ = client.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &propagationBackground})
	}()

	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		j, getErr := client.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
		if getErr != nil {
			return false, getErr
		}
		if j.Status.Succeeded > 0 {
			return true, nil
		}
		if j.Status.Failed > 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return -1, "", fmt.Errorf("wait for job %s: %w", jobName, err)
	}

	// Find the pod and get exit code and logs.
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "job-name=" + jobName})
	if err != nil {
		return -1, "", fmt.Errorf("list pods for job %s: %w", jobName, err)
	}
	if len(pods.Items) == 0 {
		return -1, "", fmt.Errorf("no pods found for job %s", jobName)
	}
	pod := pods.Items[0]
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Terminated != nil {
			exitCode = int(cs.State.Terminated.ExitCode)
			break
		}
	}
	req := client.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: "curl"})
	logBytes, logErr := req.DoRaw(ctx)
	if logErr != nil {
		logs = logErr.Error()
	} else {
		logs = string(logBytes)
	}
	return exitCode, logs, nil
}

// GetBitwardenSecretIDByKey runs a Job in-cluster that lists secrets via the Bitwarden API and returns the ID (UUID)
// of the secret whose key matches remoteKey. Used to get the UUID of a secret created by PushSecret for the
// pull-by-UUID ExternalSecret test. The Job runs in the given namespace (use BitwardenOperandNamespace so it can reach the server).
func GetBitwardenSecretIDByKey(ctx context.Context, client kubernetes.Interface, namespace, remoteKey string) (string, error) {
	baseURL := GetBitwardenSDKServerURL()
	credPath := BitwardenCredMountPath()
	script := fmt.Sprintf("TOKEN=$(cat %s/token) && ORG=$(cat %s/organization_id) && curl -k -s -X GET -H 'Content-Type: application/json' -H \"Warden-Access-Token: $TOKEN\" -d \"{\\\"organizationId\\\":\\\"$ORG\\\"}\" %s/rest/api/1/secrets", credPath, credPath, baseURL)
	code, logs, err := RunBitwardenAPIJob(ctx, client, namespace, "get-secret-id-"+GetRandomString(5), []string{"sh", "-c", script}, 2*time.Minute)
	if err != nil {
		return "", fmt.Errorf("job failed: %w (logs: %s)", err, logs)
	}
	if code != 0 {
		return "", fmt.Errorf("list secrets job exited %d: %s", code, logs)
	}
	var result struct {
		Data []struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(logs), &result); err != nil {
		return "", fmt.Errorf("parse list response: %w (logs: %s)", err, logs)
	}
	for _, s := range result.Data {
		if s.Key == remoteKey {
			return s.ID, nil
		}
	}
	return "", fmt.Errorf("secret with key %q not found in list (response had %d items)", remoteKey, len(result.Data))
}

// DeleteBitwardenSecretByKey looks up the secret with the given key in Bitwarden and deletes it via the API.
// Best-effort: no error is returned so it can be used in AfterAll cleanup without failing the suite.
// The Job runs in the given namespace (use BitwardenOperandNamespace so it can reach the server).
func DeleteBitwardenSecretByKey(ctx context.Context, client kubernetes.Interface, namespace, remoteKey string) {
	uuid, err := GetBitwardenSecretIDByKey(ctx, client, namespace, remoteKey)
	if err != nil || uuid == "" {
		return
	}
	baseURL := GetBitwardenSDKServerURL()
	credPath := BitwardenCredMountPath()
	script := fmt.Sprintf("TOKEN=$(cat %s/token) && code=$(curl -k -s -o /dev/null -w '%%{http_code}' -X DELETE -H 'Content-Type: application/json' -H \"Warden-Access-Token: $TOKEN\" -d \"{\\\"ids\\\":[\\\"%s\\\"]}\" %s/rest/api/1/secret); [ \"$code\" = \"200\" ] || exit 1", credPath, uuid, baseURL)
	_, _, _ = RunBitwardenAPIJob(ctx, client, namespace, "delete-bitwarden-secret-"+GetRandomString(5), []string{"sh", "-c", script}, 1*time.Minute)
}
