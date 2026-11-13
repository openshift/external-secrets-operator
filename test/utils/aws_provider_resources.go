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

	"github.com/aws/aws-sdk-go/aws"
	awscred "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/ssm"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

const (
	secretStoreKind = "SecretStore"
)

// AWSSecretStore returns an unstructured namespaced SecretStore for AWS.
// The service parameter should be "SecretsManager" or "ParameterStore".
// credSecretName must exist in the same namespace with keys aws_access_key_id and aws_secret_access_key.
func AWSSecretStore(name, namespace, region, service, credSecretName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": externalSecretsAPIVersionV1,
			"kind":       secretStoreKind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":       "aws-secret-store",
					"app.kubernetes.io/managed-by": "external-secrets-operator-e2e",
				},
			},
			"spec": map[string]interface{}{
				"provider": map[string]interface{}{
					"aws": map[string]interface{}{
						"service": service,
						"region":  region,
						"auth": map[string]interface{}{
							"secretRef": map[string]interface{}{
								"accessKeyIDSecretRef": map[string]interface{}{
									"name": credSecretName,
									"key":  awsCredKeyIdSecretKeyName,
								},
								"secretAccessKeySecretRef": map[string]interface{}{
									"name": credSecretName,
									"key":  awsCredAccessKeySecretKeyName,
								},
							},
						},
					},
				},
			},
		},
	}
}

// AWSExternalSecret returns an ExternalSecret with spec.data[] that syncs a single key from the store.
func AWSExternalSecret(name, namespace, storeName, targetSecretName, refreshInterval, remoteKey, secretKey, property string) *unstructured.Unstructured {
	dataEntry := map[string]interface{}{
		"secretKey": secretKey,
		"remoteRef": map[string]interface{}{
			"key": remoteKey,
		},
	}
	if property != "" {
		dataEntry["remoteRef"].(map[string]interface{})["property"] = property
	}

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
				"secretStoreRef": map[string]interface{}{
					"name": storeName,
					"kind": secretStoreKind,
				},
				"target": map[string]interface{}{
					"name": targetSecretName,
				},
				"data": []interface{}{dataEntry},
			},
		},
	}
}

// AWSExternalSecretWithDataFrom returns an ExternalSecret with spec.dataFrom[].extract.
func AWSExternalSecretWithDataFrom(name, namespace, storeName, targetSecretName, refreshInterval, remoteKey string) *unstructured.Unstructured {
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
				"secretStoreRef": map[string]interface{}{
					"name": storeName,
					"kind": secretStoreKind,
				},
				"target": map[string]interface{}{
					"name": targetSecretName,
				},
				"dataFrom": []interface{}{
					map[string]interface{}{
						"extract": map[string]interface{}{
							"key": remoteKey,
						},
					},
				},
			},
		},
	}
}

// AWSExternalSecretWithPolicy returns an ExternalSecret with a custom creationPolicy on the target.
func AWSExternalSecretWithPolicy(name, namespace, storeName, targetSecretName, refreshInterval, remoteKey, secretKey, property, creationPolicy string) *unstructured.Unstructured {
	es := AWSExternalSecret(name, namespace, storeName, targetSecretName, refreshInterval, remoteKey, secretKey, property)
	_ = unstructured.SetNestedField(es.Object, creationPolicy, "spec", "target", "creationPolicy")
	return es
}

// AWSPushSecretKey returns a PushSecret that pushes a specific key from a k8s secret to the remote store.
func AWSPushSecretKey(name, namespace, storeName, sourceSecretName, secretKey, remoteKey, refreshInterval string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": externalSecretsAPIVersionV1alpha1,
			"kind":       pushSecretKind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "external-secrets-operator-e2e",
				},
			},
			"spec": map[string]interface{}{
				"refreshInterval": refreshInterval,
				"secretStoreRefs": []interface{}{
					map[string]interface{}{
						"name": storeName,
						"kind": secretStoreKind,
					},
				},
				"selector": map[string]interface{}{
					"secret": map[string]interface{}{
						"name": sourceSecretName,
					},
				},
				"data": []interface{}{
					map[string]interface{}{
						"match": map[string]interface{}{
							"secretKey": secretKey,
							"remoteRef": map[string]interface{}{
								"remoteKey": remoteKey,
							},
						},
					},
				},
			},
		},
	}
}

// AWSPushSecretEntire returns a PushSecret that pushes all keys from a k8s secret.
func AWSPushSecretEntire(name, namespace, storeName, sourceSecretName, remoteKey, refreshInterval string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": externalSecretsAPIVersionV1alpha1,
			"kind":       pushSecretKind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "external-secrets-operator-e2e",
				},
			},
			"spec": map[string]interface{}{
				"refreshInterval": refreshInterval,
				"secretStoreRefs": []interface{}{
					map[string]interface{}{
						"name": storeName,
						"kind": secretStoreKind,
					},
				},
				"selector": map[string]interface{}{
					"secret": map[string]interface{}{
						"name": sourceSecretName,
					},
				},
				"data": []interface{}{
					map[string]interface{}{
						"match": map[string]interface{}{
							"remoteRef": map[string]interface{}{
								"remoteKey": remoteKey,
							},
						},
					},
				},
			},
		},
	}
}

// --- AWS SDK helper functions ---

func newAWSSession(ctx context.Context, k8sClient *kubernetes.Clientset, region string) (*session.Session, error) {
	id, key, err := fetchAWSCreds(ctx, k8sClient)
	if err != nil {
		return nil, fmt.Errorf("fetch AWS creds: %w", err)
	}
	return session.NewSession(&aws.Config{
		Credentials: awscred.NewCredentials(&awscred.StaticProvider{Value: awscred.Value{
			AccessKeyID:     id,
			SecretAccessKey: key,
		}}),
		Region: aws.String(region),
	})
}

// CreateAWSSecret creates a new secret in AWS Secrets Manager.
func CreateAWSSecret(ctx context.Context, k8sClient *kubernetes.Clientset, awsSecretName, secretValue, region string) error {
	sess, err := newAWSSession(ctx, k8sClient, region)
	if err != nil {
		return err
	}
	svc := secretsmanager.New(sess)
	_, err = svc.CreateSecretWithContext(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(awsSecretName),
		SecretString: aws.String(secretValue),
	})
	return err
}

// UpdateAWSSecretKeyValue updates a key-value pair in a JSON AWS Secrets Manager secret.
func UpdateAWSSecretKeyValue(ctx context.Context, k8sClient *kubernetes.Clientset, awsSecretName, key, value, region string) error {
	sess, err := newAWSSession(ctx, k8sClient, region)
	if err != nil {
		return err
	}
	svc := secretsmanager.New(sess)

	out, err := svc.GetSecretValueWithContext(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(awsSecretName),
	})
	if err != nil {
		return fmt.Errorf("get secret for update: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(aws.StringValue(out.SecretString)), &data); err != nil {
		return fmt.Errorf("unmarshal secret: %w", err)
	}
	data[key] = value
	updated, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal updated secret: %w", err)
	}

	_, err = svc.PutSecretValueWithContext(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(awsSecretName),
		SecretString: aws.String(string(updated)),
	})
	return err
}

// GetAWSSecretValue returns the secret string from AWS Secrets Manager.
func GetAWSSecretValue(ctx context.Context, k8sClient *kubernetes.Clientset, awsSecretName, region string) (string, error) {
	sess, err := newAWSSession(ctx, k8sClient, region)
	if err != nil {
		return "", err
	}
	svc := secretsmanager.New(sess)
	out, err := svc.GetSecretValueWithContext(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(awsSecretName),
	})
	if err != nil {
		return "", err
	}
	return aws.StringValue(out.SecretString), nil
}

// PutAWSSSMParameter creates or updates an AWS Systems Manager Parameter Store parameter.
func PutAWSSSMParameter(ctx context.Context, k8sClient *kubernetes.Clientset, paramName, value, region string) error {
	sess, err := newAWSSession(ctx, k8sClient, region)
	if err != nil {
		return err
	}
	svc := ssm.New(sess)
	_, err = svc.PutParameterWithContext(ctx, &ssm.PutParameterInput{
		Name:      aws.String(paramName),
		Value:     aws.String(value),
		Type:      aws.String("String"),
		Overwrite: aws.Bool(true),
	})
	return err
}

// GetAWSSSMParameter returns the value of an SSM parameter.
func GetAWSSSMParameter(ctx context.Context, k8sClient *kubernetes.Clientset, paramName, region string) (string, error) {
	sess, err := newAWSSession(ctx, k8sClient, region)
	if err != nil {
		return "", err
	}
	svc := ssm.New(sess)
	out, err := svc.GetParameterWithContext(ctx, &ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", err
	}
	return aws.StringValue(out.Parameter.Value), nil
}

// DeleteAWSSSMParameter deletes an SSM parameter.
func DeleteAWSSSMParameter(ctx context.Context, k8sClient *kubernetes.Clientset, paramName, region string) error {
	sess, err := newAWSSession(ctx, k8sClient, region)
	if err != nil {
		return err
	}
	svc := ssm.New(sess)
	_, err = svc.DeleteParameterWithContext(ctx, &ssm.DeleteParameterInput{
		Name: aws.String(paramName),
	})
	return err
}

// CopyAWSCredsToNamespace copies the aws-creds secret from kube-system to the target namespace.
func CopyAWSCredsToNamespace(ctx context.Context, k8sClient *kubernetes.Clientset, targetNamespace, targetSecretName string) error {
	src, err := k8sClient.CoreV1().Secrets(awsCredNamespace).Get(ctx, awsCredSecretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get source aws-creds secret: %w", err)
	}

	dst := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetSecretName,
			Namespace: targetNamespace,
		},
		Data: map[string][]byte{
			awsCredKeyIdSecretKeyName:     src.Data[awsCredKeyIdSecretKeyName],
			awsCredAccessKeySecretKeyName: src.Data[awsCredAccessKeySecretKeyName],
		},
	}
	_, err = k8sClient.CoreV1().Secrets(targetNamespace).Create(ctx, dst, metav1.CreateOptions{})
	return err
}
