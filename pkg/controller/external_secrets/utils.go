package external_secrets

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"go.uber.org/zap/zapcore"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

func getNamespace(esc *operatorv1alpha1.ExternalSecretsConfig) string {
	if esc.Spec.ControllerConfig.Namespace != "" {
		return esc.Spec.ControllerConfig.Namespace
	}
	return externalsecretsDefaultNamespace
}

func updateNamespace(obj client.Object, esc *operatorv1alpha1.ExternalSecretsConfig) {
	obj.SetNamespace(getNamespace(esc))
}

func containsProcessedAnnotation(esc *operatorv1alpha1.ExternalSecretsConfig) bool {
	_, exist := esc.GetAnnotations()[controllerProcessedAnnotation]
	return exist
}

func addProcessedAnnotation(esc *operatorv1alpha1.ExternalSecretsConfig) bool {
	annotations := esc.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}
	if _, exist := annotations[controllerProcessedAnnotation]; !exist {
		annotations[controllerProcessedAnnotation] = "true"
		esc.SetAnnotations(annotations)
		return true
	}
	return false
}

func (r *Reconciler) updateCondition(esc *operatorv1alpha1.ExternalSecretsConfig, prependErr error) error {
	if err := r.updateStatus(r.ctx, esc); err != nil {
		errUpdate := fmt.Errorf("failed to update %s/%s status: %w", esc.GetNamespace(), esc.GetName(), err)
		if prependErr != nil {
			return utilerrors.NewAggregate([]error{err, errUpdate})
		}
		return errUpdate
	}
	return prependErr
}

// updateStatus is for updating the status subresource of externalsecretsconfig.openshift.operator.io.
func (r *Reconciler) updateStatus(ctx context.Context, changed *operatorv1alpha1.ExternalSecretsConfig) error {
	namespacedName := types.NamespacedName{Name: changed.Name, Namespace: changed.Namespace}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		r.log.V(4).Info("updating externalsecretsconfig.openshift.operator.io status", "request", namespacedName)
		current := &operatorv1alpha1.ExternalSecretsConfig{}
		if err := r.Get(ctx, namespacedName, current); err != nil {
			return fmt.Errorf("failed to fetch externalsecretsconfig.openshift.operator.io %q for status update: %w", namespacedName, err)
		}
		changed.Status.DeepCopyInto(&current.Status)

		if err := r.StatusUpdate(ctx, current); err != nil {
			return fmt.Errorf("failed to update externalsecretsconfig.openshift.operator.io %q status: %w", namespacedName, err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// validateExternalSecretsConfig is for validating the ExternalSecretsConfig CR fields, apart from the
// CEL validations present in CRD.
func (r *Reconciler) validateExternalSecretsConfig(esc *operatorv1alpha1.ExternalSecretsConfig) error {
	if isCertManagerConfigEnabled(esc) {
		if _, ok := r.optionalResourcesList[certificateCRDGKV]; !ok {
			return fmt.Errorf("spec.certManagerConfig.enabled is set, but cert-manager is not installed")
		}
	}
	return nil
}

// isCertManagerConfigEnabled returns whether CertManagerConfig is enabled in ExternalSecretsConfig CR Spec.
func isCertManagerConfigEnabled(esc *operatorv1alpha1.ExternalSecretsConfig) bool {
	return esc.Spec.ApplicationConfig.CertManagerConfig != nil && common.EvalMode(esc.Spec.ApplicationConfig.CertManagerConfig.Mode)
}

// isBitwardenConfigEnabled returns whether CertManagerConfig is enabled in ExternalSecretsConfig CR Spec.
func isBitwardenConfigEnabled(esc *operatorv1alpha1.ExternalSecretsConfig) bool {
	return esc.Spec.ApplicationConfig.BitwardenSecretManagerProvider != nil && common.EvalMode(esc.Spec.ApplicationConfig.BitwardenSecretManagerProvider.Mode)
}

func getLogLevel(config operatorv1alpha1.ExternalSecretsConfigSpec) string {
	switch config.ApplicationConfig.LogLevel {
	case 0, 1, 2:
		return zapcore.Level(config.ApplicationConfig.LogLevel).String()
	case 4, 5:
		return zapcore.DebugLevel.String()
	}
	return zapcore.InfoLevel.String()
}

func getOperatingNamespace(esc *operatorv1alpha1.ExternalSecretsConfig) string {
	return esc.Spec.ApplicationConfig.OperatingNamespace
}

func (r *Reconciler) IsCertManagerInstalled() bool {
	_, ok := r.optionalResourcesList[certificateCRDGKV]
	return ok
}
