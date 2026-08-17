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

const (
	generatorsAPIVersionV1alpha1 = "generators.external-secrets.io/v1alpha1"
	clusterGeneratorKind         = "ClusterGenerator"
	passwordGeneratorKind        = "Password"
)

// PasswordClusterGenerator returns an unstructured ClusterGenerator of kind Password.
func PasswordClusterGenerator(name string, length, digits, symbols int, symbolCharacters string, allowRepeat bool) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": generatorsAPIVersionV1alpha1,
			"kind":       clusterGeneratorKind,
			"metadata": map[string]interface{}{
				"name": name,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "external-secrets-operator-e2e",
				},
			},
			"spec": map[string]interface{}{
				"kind": passwordGeneratorKind,
				"generator": map[string]interface{}{
					"passwordSpec": map[string]interface{}{
						"length":           int64(length),
						"digits":           int64(digits),
						"symbols":          int64(symbols),
						"symbolCharacters": symbolCharacters,
						"allowRepeat":      allowRepeat,
					},
				},
			},
		},
	}
}

// PasswordNamespacedGenerator returns an unstructured namespaced Password generator.
func PasswordNamespacedGenerator(name, namespace string, length, digits, symbols int, symbolCharacters string, allowRepeat bool) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": generatorsAPIVersionV1alpha1,
			"kind":       passwordGeneratorKind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "external-secrets-operator-e2e",
				},
			},
			"spec": map[string]interface{}{
				"length":           int64(length),
				"digits":           int64(digits),
				"symbols":          int64(symbols),
				"symbolCharacters": symbolCharacters,
				"allowRepeat":      allowRepeat,
			},
		},
	}
}

// ExternalSecretWithGenerator returns an ExternalSecret that uses a generatorRef in dataFrom.
func ExternalSecretWithGenerator(name, namespace, targetSecretName, generatorName, generatorKind, refreshInterval string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": externalSecretsAPIVersionV1,
			"kind":       externalSecretKind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "external-secrets-operator-e2e",
				},
			},
			"spec": map[string]interface{}{
				"refreshInterval": refreshInterval,
				"target": map[string]interface{}{
					"name": targetSecretName,
				},
				"dataFrom": []interface{}{
					map[string]interface{}{
						"sourceRef": map[string]interface{}{
							"generatorRef": map[string]interface{}{
								"apiVersion": generatorsAPIVersionV1alpha1,
								"kind":       generatorKind,
								"name":       generatorName,
							},
						},
					},
				},
			},
		},
	}
}
