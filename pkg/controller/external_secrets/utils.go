package external_secrets

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"go.uber.org/zap/zapcore"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

// addFinalizer adds finalizer to externalsecrets.openshift.operator.io resource.
func (r *Reconciler) addFinalizer(ctx context.Context, externalsecrets *operatorv1alpha1.ExternalSecrets) error {
	namespacedName := types.NamespacedName{Name: externalsecrets.Name, Namespace: externalsecrets.Namespace}
	if !controllerutil.ContainsFinalizer(externalsecrets, finalizer) {
		if !controllerutil.AddFinalizer(externalsecrets, finalizer) {
			return fmt.Errorf("failed to create %q externalsecrets.openshift.operator.io object with finalizers added", namespacedName)
		}

		// update externalsecrets.openshift.operator.io on adding finalizer.
		if err := r.UpdateWithRetry(ctx, externalsecrets); err != nil {
			return fmt.Errorf("failed to add finalizers on %q externalsecrets.openshift.operator.io with %w", namespacedName, err)
		}

		updated := &operatorv1alpha1.ExternalSecrets{}
		if err := r.Get(ctx, namespacedName, updated); err != nil {
			return fmt.Errorf("failed to fetch externalsecrets.openshift.operator.io %q after updating finalizers: %w", namespacedName, err)
		}
		updated.DeepCopyInto(externalsecrets)
		return nil
	}
	return nil
}

// removeFinalizer removes finalizers added to externalsecrets.openshift.operator.io resource.
func (r *Reconciler) removeFinalizer(ctx context.Context, externalsecrets *operatorv1alpha1.ExternalSecrets, finalizer string) error {
	namespacedName := types.NamespacedName{Name: externalsecrets.Name, Namespace: externalsecrets.Namespace}
	if controllerutil.ContainsFinalizer(externalsecrets, finalizer) {
		if !controllerutil.RemoveFinalizer(externalsecrets, finalizer) {
			return fmt.Errorf("failed to create %q externalsecrets.openshift.operator.io object with finalizers removed", namespacedName)
		}

		if err := r.UpdateWithRetry(ctx, externalsecrets); err != nil {
			return fmt.Errorf("failed to remove finalizers on %q externalsecrets.openshift.operator.io with %w", namespacedName, err)
		}
		return nil
	}

	return nil
}

func getNamespace(es *operatorv1alpha1.ExternalSecrets) string {
	ns := externalsecretsDefaultNamespace
	if es.Spec.ControllerConfig != nil && es.Spec.ControllerConfig.Namespace != "" {
		ns = es.Spec.ControllerConfig.Namespace
	}
	return ns
}

func updateNamespace(obj client.Object, es *operatorv1alpha1.ExternalSecrets) {
	obj.SetNamespace(getNamespace(es))
}

func containsProcessedAnnotation(externalsecrets *operatorv1alpha1.ExternalSecrets) bool {
	_, exist := externalsecrets.GetAnnotations()[controllerProcessedAnnotation]
	return exist
}

func addProcessedAnnotation(externalsecrets *operatorv1alpha1.ExternalSecrets) bool {
	annotations := externalsecrets.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}
	if _, exist := annotations[controllerProcessedAnnotation]; !exist {
		annotations[controllerProcessedAnnotation] = "true"
		externalsecrets.SetAnnotations(annotations)
		return true
	}
	return false
}

func (r *Reconciler) updateCondition(externalsecrets *operatorv1alpha1.ExternalSecrets, prependErr error) error {
	if err := r.updateStatus(r.ctx, externalsecrets); err != nil {
		errUpdate := fmt.Errorf("failed to update %s/%s status: %w", externalsecrets.GetNamespace(), externalsecrets.GetName(), err)
		if prependErr != nil {
			return utilerrors.NewAggregate([]error{err, errUpdate})
		}
		return errUpdate
	}
	return prependErr
}

// updateStatus is for updating the status subresource of externalsecrets.openshift.operator.io.
func (r *Reconciler) updateStatus(ctx context.Context, changed *operatorv1alpha1.ExternalSecrets) error {
	namespacedName := types.NamespacedName{Name: changed.Name, Namespace: changed.Namespace}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		r.log.V(4).Info("updating externalsecrets.openshift.operator.io status", "request", namespacedName)
		current := &operatorv1alpha1.ExternalSecrets{}
		if err := r.Get(ctx, namespacedName, current); err != nil {
			return fmt.Errorf("failed to fetch externalsecrets.openshift.operator.io %q for status update: %w", namespacedName, err)
		}
		changed.Status.DeepCopyInto(&current.Status)

		if err := r.StatusUpdate(ctx, current); err != nil {
			return fmt.Errorf("failed to update externalsecrets.openshift.operator.io %q status: %w", namespacedName, err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// validateExternalSecretsConfig is for validating the ExternalSecrets CR fields, apart from the
// CEL validations present in CRD.
func (r *Reconciler) validateExternalSecretsConfig(es *operatorv1alpha1.ExternalSecrets) error {
	if isCertManagerConfigEnabled(es) {
		if _, ok := r.optionalResourcesList[certificateCRDGKV]; !ok {
			return fmt.Errorf("spec.externalSecretsConfig.webhookConfig.certManagerConfig.enabled is set, but cert-manager is not installed")
		}

	}
	return nil
}

// isCertManagerConfigEnabled returns whether CertManagerConfig is enabled in ExternalSecrets CR Spec.
func isCertManagerConfigEnabled(es *operatorv1alpha1.ExternalSecrets) bool {
	return es.Spec != (operatorv1alpha1.ExternalSecretsSpec{}) && es.Spec.ExternalSecretsConfig != nil &&
		es.Spec.ExternalSecretsConfig.WebhookConfig != nil &&
		es.Spec.ExternalSecretsConfig.WebhookConfig.CertManagerConfig != nil &&
		common.ParseBool(es.Spec.ExternalSecretsConfig.WebhookConfig.CertManagerConfig.Enabled)
}

// isBitwardenConfigEnabled returns whether CertManagerConfig is enabled in ExternalSecrets CR Spec.
func isBitwardenConfigEnabled(es *operatorv1alpha1.ExternalSecrets) bool {
	return es.Spec != (operatorv1alpha1.ExternalSecretsSpec{}) && es.Spec.ExternalSecretsConfig != nil && es.Spec.ExternalSecretsConfig.BitwardenSecretManagerProvider != nil &&
		common.ParseBool(es.Spec.ExternalSecretsConfig.BitwardenSecretManagerProvider.Enabled)
}

func getLogLevel(config *operatorv1alpha1.ExternalSecretsConfig) string {
	if config != nil {
		return zapcore.Level(config.LogLevel).String()
	}
	return "info"
}

func getOperatingNamespace(externalsecrets *operatorv1alpha1.ExternalSecrets) string {
	if externalsecrets == nil || externalsecrets.Spec.ExternalSecretsConfig == nil {
		return ""
	}
	return externalsecrets.Spec.ExternalSecretsConfig.OperatingNamespace
}
