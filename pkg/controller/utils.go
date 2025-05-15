package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/external-secrets-operator/api/v1alpha1"
)

// addFinalizer adds finalizer to external-secrets.openshift.operator.io resource.
func (r *ExternalSecretsReconciler) addFinalizer(ctx context.Context, externalsecrets *v1alpha1.ExternalSecretsOperator) error {
	namespacedName := types.NamespacedName{Name: externalsecrets.Name, Namespace: externalsecrets.Namespace}
	if !controllerutil.ContainsFinalizer(externalsecrets, finalizer) {
		if !controllerutil.AddFinalizer(externalsecrets, finalizer) {
			return fmt.Errorf("failed to create %q external-secrets.openshift.operator.io object with finalizers added", namespacedName)
		}

		// update external-secrets.openshift.operator.io on adding finalizer.
		if err := r.UpdateWithRetry(ctx, externalsecrets); err != nil {
			return fmt.Errorf("failed to add finalizers on %q external-secrets.openshift.operator.io with %w", namespacedName, err)
		}

		updated := &v1alpha1.ExternalSecretsOperator{}
		if err := r.Get(ctx, namespacedName, updated); err != nil {
			return fmt.Errorf("failed to fetch external-secrets.openshift.operator.io %q after updating finalizers: %w", namespacedName, err)
		}
		updated.DeepCopyInto(externalsecrets)
		return nil
	}
	return nil
}

// removeFinalizer removes finalizers added to external-secrets.openshift.operator.io resource.
func (r *ExternalSecretsReconciler) removeFinalizer(ctx context.Context, externalsecrets *v1alpha1.ExternalSecretsOperator, finalizer string) error {
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

func containsProcessedAnnotation(externalsecrets *v1alpha1.ExternalSecretsOperator) bool {
	_, exist := externalsecrets.GetAnnotations()[controllerProcessedAnnotation]
	return exist
}

func (r *ExternalSecretsReconciler) updateCondition(externalsecrets *v1alpha1.ExternalSecretsOperator, prependErr error) error {
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
func (r *ExternalSecretsReconciler) updateStatus(ctx context.Context, changed *v1alpha1.ExternalSecretsOperator) error {
	namespacedName := types.NamespacedName{Name: changed.Name, Namespace: changed.Namespace}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		r.log.V(4).Info("updating external-secrets.openshift.operator.io status", "request", namespacedName)
		current := &v1alpha1.ExternalSecretsOperator{}
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
