package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/openshift/external-secrets-operator/api/v1alpha1"
)

var (
	//scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(runtime.NewScheme())
)

// addFinalizer adds finalizer to external-secrets.openshift.operator.io resource.
func (r *ExternalSecretsReconciler) addFinalizer(ctx context.Context, externalsecrets *v1alpha1.ExternalSecrets) error {
	namespacedName := types.NamespacedName{Name: externalsecrets.Name, Namespace: externalsecrets.Namespace}
	if !controllerutil.ContainsFinalizer(externalsecrets, finalizer) {
		if !controllerutil.AddFinalizer(externalsecrets, finalizer) {
			return fmt.Errorf("failed to create %q external-secrets.openshift.operator.io object with finalizers added", namespacedName)
		}

		// update external-secrets.openshift.operator.io on adding finalizer.
		if err := r.UpdateWithRetry(ctx, externalsecrets); err != nil {
			return fmt.Errorf("failed to add finalizers on %q external-secrets.openshift.operator.io with %w", namespacedName, err)
		}

		updated := &v1alpha1.ExternalSecrets{}
		if err := r.Get(ctx, namespacedName, updated); err != nil {
			return fmt.Errorf("failed to fetch external-secrets.openshift.operator.io %q after updating finalizers: %w", namespacedName, err)
		}
		updated.DeepCopyInto(externalsecrets)
		return nil
	}
	return nil
}

// removeFinalizer removes finalizers added to external-secrets.openshift.operator.io resource.
func (r *ExternalSecretsReconciler) removeFinalizer(ctx context.Context, externalsecrets *v1alpha1.ExternalSecrets, finalizer string) error {
	namespacedName := types.NamespacedName{Name: externalsecrets.Name, Namespace: externalsecrets.Namespace}
	if controllerutil.ContainsFinalizer(externalsecrets, finalizer) {
		if !controllerutil.RemoveFinalizer(externalsecrets, finalizer) {
			return fmt.Errorf("failed to create %q external-secrets.openshift.operator.io object with finalizers removed", namespacedName)
		}

		if err := r.UpdateWithRetry(ctx, externalsecrets); err != nil {
			return fmt.Errorf("failed to remove finalizers on %q external-secrets.openshift.operator.io with %w", namespacedName, err)
		}
		return nil
	}

	return nil
}

func containsProcessedAnnotation(externalsecrets *v1alpha1.ExternalSecrets) bool {
	_, exist := externalsecrets.GetAnnotations()[controllerProcessedAnnotation]
	return exist
}

func (r *ExternalSecretsReconciler) updateCondition(externalsecrets *v1alpha1.ExternalSecrets, prependErr error) error {
	if err := r.updateStatus(r.ctx, externalsecrets); err != nil {
		errUpdate := fmt.Errorf("failed to update %s/%s status: %w", externalsecrets.GetNamespace(), externalsecrets.GetName(), err)
		if prependErr != nil {
			return utilerrors.NewAggregate([]error{err, errUpdate})
		}
		return errUpdate
	}
	return prependErr
}

// updateStatus is for updating the status subresource of external-secrets.openshift.operator.io.
func (r *ExternalSecretsReconciler) updateStatus(ctx context.Context, changed *v1alpha1.ExternalSecrets) error {
	namespacedName := types.NamespacedName{Name: changed.Name, Namespace: changed.Namespace}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		r.log.V(4).Info("updating external-secrets.openshift.operator.io status", "request", namespacedName)
		current := &v1alpha1.ExternalSecrets{}
		if err := r.Get(ctx, namespacedName, current); err != nil {
			return fmt.Errorf("failed to fetch external-secrets.openshift.operator.io %q for status update: %w", namespacedName, err)
		}
		changed.Status.DeepCopyInto(&current.Status)

		if err := r.StatusUpdate(ctx, current); err != nil {
			return fmt.Errorf("failed to update external-secrets.openshift.operator.io %q status: %w", namespacedName, err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func decodeRoleBindingObjBytes(objBytes []byte) *rbacv1.RoleBinding {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*rbacv1.RoleBinding)
}

func decodeServiceObjBytes(objBytes []byte) *corev1.Service {
	obj, err := runtime.Decode(codecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*corev1.Service)
}

func decodeServiceAccountObjBytes(objBytes []byte) *corev1.ServiceAccount {
	obj, err := runtime.Decode(codecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*corev1.ServiceAccount)
}

func decodeCertificateObjBytes(objBytes []byte) *certmanagerv1.Certificate {
	obj, err := runtime.Decode(codecs.UniversalDecoder(certmanagerv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*certmanagerv1.Certificate)
}

func updateNamespace(obj client.Object, newNamespace string) {
	obj.SetNamespace(newNamespace)
}

func updateResourceLabels(obj client.Object, labels map[string]string) {
	obj.SetLabels(labels)
}
