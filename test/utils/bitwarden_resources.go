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
	externalSecretsAPIVersionV1       = "external-secrets.io/v1"
	externalSecretsAPIVersionV1alpha1 = "external-secrets.io/v1alpha1"
	clusterSecretStoreKind            = "ClusterSecretStore"
	externalSecretKind                = "ExternalSecret"
	pushSecretKind                    = "PushSecret"
)

// BitwardenClusterSecretStore returns an unstructured ClusterSecretStore for the Bitwarden provider.
func BitwardenClusterSecretStore(name, credSecretName, credNamespace, sdkURL, caBundle, orgID, projectID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": externalSecretsAPIVersionV1,
			"kind":       clusterSecretStoreKind,
			"metadata": map[string]interface{}{
				"name": name,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":       "bitwarden-secret-store",
					"app.kubernetes.io/managed-by": "external-secrets-operator-e2e",
				},
			},
			"spec": map[string]interface{}{
				"provider": map[string]interface{}{
					"bitwardensecretsmanager": map[string]interface{}{
						"auth": map[string]interface{}{
							"secretRef": map[string]interface{}{
								"credentials": map[string]interface{}{
									"key":       TokenSecretKey,
									"name":      credSecretName,
									"namespace": credNamespace,
								},
							},
						},
						"bitwardenServerSDKURL": sdkURL,
						"caBundle":             caBundle,
						"organizationID":       orgID,
						"projectID":            projectID,
					},
				},
			},
		},
	}
}

// BitwardenExternalSecretByName returns an unstructured ExternalSecret that pulls by secret name (key).
func BitwardenExternalSecretByName(name, namespace, targetSecretName, storeName, remoteKey string) *unstructured.Unstructured {
	u := BitwardenExternalSecretBase(name, namespace, targetSecretName, storeName)
	_ = unstructured.SetNestedField(u.Object, []interface{}{
		map[string]interface{}{
			"secretKey": "value",
			"remoteRef": map[string]interface{}{
				"key": remoteKey,
			},
		},
	}, "spec", "data")
	return u
}

// BitwardenExternalSecretByUUID returns an unstructured ExternalSecret that pulls by secret UUID.
func BitwardenExternalSecretByUUID(name, namespace, targetSecretName, storeName, secretUUID string) *unstructured.Unstructured {
	u := BitwardenExternalSecretBase(name, namespace, targetSecretName, storeName)
	_ = unstructured.SetNestedField(u.Object, []interface{}{
		map[string]interface{}{
			"secretKey": "value",
			"remoteRef": map[string]interface{}{
				"key": secretUUID,
			},
		},
	}, "spec", "data")
	return u
}

func BitwardenExternalSecretBase(name, namespace, targetSecretName, storeName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": externalSecretsAPIVersionV1,
			"kind":       externalSecretKind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":       "bitwarden-external-secret",
					"app.kubernetes.io/managed-by": "external-secrets-operator-e2e",
				},
			},
			"spec": map[string]interface{}{
				"refreshInterval": "1h",
				"secretStoreRef": map[string]interface{}{
					"name": storeName,
					"kind": clusterSecretStoreKind,
				},
				"target": map[string]interface{}{
					"name":           targetSecretName,
					"creationPolicy": "Owner",
				},
			},
		},
	}
}

// BitwardenPushSecret returns an unstructured PushSecret.
// Each spec.data entry must have a required "match" with secretKey and remoteRef (see PushSecret CRD).
func BitwardenPushSecret(name, namespace, storeName, sourceSecretName, remoteKey, note string) *unstructured.Unstructured {
	dataEntry := map[string]interface{}{
		"match": map[string]interface{}{
			"secretKey": "value",
			"remoteRef": map[string]interface{}{
				"remoteKey": remoteKey,
			},
		},
	}
	if note != "" {
		dataEntry["metadata"] = map[string]interface{}{
			"note": note,
		}
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": externalSecretsAPIVersionV1alpha1,
			"kind":       pushSecretKind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":       "bitwarden-push-secret",
					"app.kubernetes.io/managed-by": "external-secrets-operator-e2e",
				},
			},
			"spec": map[string]interface{}{
				"refreshInterval": "1h",
				"secretStoreRefs": []interface{}{
					map[string]interface{}{
						"name": storeName,
						"kind": clusterSecretStoreKind,
					},
				},
				"selector": map[string]interface{}{
					"secret": map[string]interface{}{
						"name": sourceSecretName,
					},
				},
				"data": []interface{}{dataEntry},
			},
		},
	}
}
