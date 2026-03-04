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

	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

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
