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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	operandNamespaceName = "external-secrets"
	operandLabelSelector = "app=external-secrets"

	externalSecretsConfigName = "cluster"
)

// CleanupESOOperandAndRelated removes operand CR instances (by listing CRDs with app=external-secrets),
// the cluster ExternalSecretsConfig instance, webhooks, ClusterRoles/ClusterRoleBindings, and the
// operand namespace. CRDs are not deleted so reruns can reuse the same cluster. Best-effort; errors are ignored.
func CleanupESOOperandAndRelated(ctx context.Context, cfg *rest.Config) {
	if cfg == nil {
		return
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return
	}
	extClient, err := apiextensionsclientset.NewForConfig(cfg)
	if err != nil {
		return
	}
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return
	}

	// 1. Delete all instances of operand CRDs (label app=external-secrets).
	crdList, _ := extClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{LabelSelector: operandLabelSelector})
	for i := range crdList.Items {
		crd := &crdList.Items[i]
		version := preferredVersion(crd)
		if version == "" {
			continue
		}
		gvr := schema.GroupVersionResource{
			Group:    crd.Spec.Group,
			Version:  version,
			Resource: crd.Spec.Names.Plural,
		}
		if crd.Spec.Scope == apiextensionsv1.ClusterScoped {
			list, err := dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			for j := range list.Items {
				_ = dynamicClient.Resource(gvr).Delete(ctx, list.Items[j].GetName(), metav1.DeleteOptions{})
			}
		} else {
			list, err := dynamicClient.Resource(gvr).Namespace(operandNamespaceName).List(ctx, metav1.ListOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					continue // namespace already gone
				}
				continue
			}
			for j := range list.Items {
				_ = dynamicClient.Resource(gvr).Namespace(operandNamespaceName).Delete(ctx, list.Items[j].GetName(), metav1.DeleteOptions{})
			}
		}
	}

	// 2. Delete the cluster ExternalSecretsConfig instance (operator.openshift.io).
	escGVR := schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1alpha1", Resource: "externalsecretsconfigs"}
	_ = dynamicClient.Resource(escGVR).Delete(ctx, externalSecretsConfigName, metav1.DeleteOptions{})

	// 3. Remove webhooks first so namespace deletion can proceed.
	webhooks, _ := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, metav1.ListOptions{LabelSelector: operandLabelSelector})
	for i := range webhooks.Items {
		_ = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(ctx, webhooks.Items[i].Name, metav1.DeleteOptions{})
	}

	// 4. ClusterRoleBindings before ClusterRoles.
	crbs, _ := clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{LabelSelector: operandLabelSelector})
	for i := range crbs.Items {
		_ = clientset.RbacV1().ClusterRoleBindings().Delete(ctx, crbs.Items[i].Name, metav1.DeleteOptions{})
	}
	crList, _ := clientset.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{LabelSelector: operandLabelSelector})
	for i := range crList.Items {
		_ = clientset.RbacV1().ClusterRoles().Delete(ctx, crList.Items[i].Name, metav1.DeleteOptions{})
	}

	// 5. Operand namespace.
	_ = clientset.CoreV1().Namespaces().Delete(ctx, operandNamespaceName, metav1.DeleteOptions{})
}

// preferredVersion returns the stored/serving version for the CRD (first version with Storage: true, else first version).
func preferredVersion(crd *apiextensionsv1.CustomResourceDefinition) string {
	for _, v := range crd.Spec.Versions {
		if v.Storage {
			return v.Name
		}
	}
	if len(crd.Spec.Versions) > 0 {
		return crd.Spec.Versions[0].Name
	}
	return ""
}
