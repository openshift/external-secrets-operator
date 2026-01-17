package external_secrets

import (
	"context"
	"fmt"
	"os"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"go.uber.org/zap/zapcore"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

func getNamespace(_ *operatorv1alpha1.ExternalSecretsConfig) string {
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

// updateStatus is for updating the status subresource of externalsecretsconfigs.operator.openshift.io.
func (r *Reconciler) updateStatus(ctx context.Context, changed *operatorv1alpha1.ExternalSecretsConfig) error {
	namespacedName := client.ObjectKeyFromObject(changed)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		r.log.V(4).Info("updating externalsecretsconfigs.operator.openshift.io status", "request", namespacedName)
		current := &operatorv1alpha1.ExternalSecretsConfig{}
		if err := r.Get(ctx, namespacedName, current); err != nil {
			return fmt.Errorf("failed to fetch externalsecretsconfigs.operator.openshift.io %q for status update: %w", namespacedName, err)
		}
		changed.Status.DeepCopyInto(&current.Status)

		if err := r.StatusUpdate(ctx, current); err != nil {
			return fmt.Errorf("failed to update externalsecretsconfigs.operator.openshift.io %q status: %w", namespacedName, err)
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
			return fmt.Errorf("spec.controllerConfig.certProvider.certManager.mode is set, but cert-manager is not installed")
		}
	}
	return nil
}

// isCertManagerConfigEnabled returns whether CertManagerConfig is enabled in ExternalSecretsConfig CR Spec.
func isCertManagerConfigEnabled(esc *operatorv1alpha1.ExternalSecretsConfig) bool {
	return esc.Spec.ControllerConfig.CertProvider != nil &&
		esc.Spec.ControllerConfig.CertProvider.CertManager != nil &&
		common.EvalMode(esc.Spec.ControllerConfig.CertProvider.CertManager.Mode)
}

// isBitwardenConfigEnabled returns whether BitwardenSecretManagerProvider is enabled in ExternalSecretsConfig CR Spec.
func isBitwardenConfigEnabled(esc *operatorv1alpha1.ExternalSecretsConfig) bool {
	return esc.Spec.Plugins.BitwardenSecretManagerProvider != nil &&
		common.EvalMode(esc.Spec.Plugins.BitwardenSecretManagerProvider.Mode)
}

func getLogLevel(esc *operatorv1alpha1.ExternalSecretsConfig, esm *operatorv1alpha1.ExternalSecretsManager) string {
	var logLevel int32 = 1
	if esc.Spec.ApplicationConfig.LogLevel != 0 {
		logLevel = esc.Spec.ApplicationConfig.LogLevel
	} else if esm.Spec.GlobalConfig != nil && esm.Spec.GlobalConfig.LogLevel != 0 {
		logLevel = esm.Spec.GlobalConfig.LogLevel
	}
	switch logLevel {
	case 0, 1, 2:
		return zapcore.Level(logLevel).String()
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

// getProxyConfiguration returns the proxy configuration based on precedence.
// The precedence order is: ExternalSecretsConfig > ExternalSecretsManager > OLM environment variables.
func (r *Reconciler) getProxyConfiguration(esc *operatorv1alpha1.ExternalSecretsConfig) *operatorv1alpha1.ProxyConfig {
	var proxyConfig *operatorv1alpha1.ProxyConfig

	// Check ExternalSecretsConfig first
	switch {
	case esc.Spec.ApplicationConfig.Proxy != nil:
		proxyConfig = esc.Spec.ApplicationConfig.Proxy
	case r.esm.Spec.GlobalConfig != nil && r.esm.Spec.GlobalConfig.Proxy != nil:
		// Check ExternalSecretsManager second
		proxyConfig = r.esm.Spec.GlobalConfig.Proxy
	default:
		// Fall back to OLM environment variables
		olmHTTPProxy := os.Getenv(httpProxyEnvVar)
		olmHTTPSProxy := os.Getenv(httpsProxyEnvVar)
		olmNoProxy := os.Getenv(noProxyEnvVar)

		// Only create proxy config if at least one OLM env var is set
		if olmHTTPProxy != "" || olmHTTPSProxy != "" || olmNoProxy != "" {
			proxyConfig = &operatorv1alpha1.ProxyConfig{
				HTTPProxy:  olmHTTPProxy,
				HTTPSProxy: olmHTTPSProxy,
				NoProxy:    olmNoProxy,
			}
		}
	}

	return proxyConfig
}
