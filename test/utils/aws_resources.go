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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// AWSClusterSecretStore returns an unstructured ClusterSecretStore for the AWS provider (Secrets Manager).
// Credentials are read from the fixed secret awsCredSecretName in awsCredNamespace (see conditions.go).
// Used by the cross-platform e2e suite (e.g. GCP cluster accessing AWS Secrets Manager).
func AWSClusterSecretStore(storeName, region string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": externalSecretsAPIVersionV1,
			"kind":       clusterSecretStoreKind,
			"metadata": map[string]interface{}{
				"name": storeName,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":       "aws-secret-store",
					"app.kubernetes.io/managed-by": "external-secrets-operator-e2e",
				},
			},
			"spec": map[string]interface{}{
				"provider": map[string]interface{}{
					"aws": map[string]interface{}{
						"service": "SecretsManager",
						"region":  region,
						"auth": map[string]interface{}{
							"secretRef": map[string]interface{}{
								"accessKeyIDSecretRef": map[string]interface{}{
									"name":      awsCredSecretName,
									"key":       awsCredKeyIdSecretKeyName,
									"namespace": awsCredNamespace,
								},
								"secretAccessKeySecretRef": map[string]interface{}{
									"name":      awsCredSecretName,
									"key":       awsCredAccessKeySecretKeyName,
									"namespace": awsCredNamespace,
								},
							},
						},
					},
				},
			},
		},
	}
}
