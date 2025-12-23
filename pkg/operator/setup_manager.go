package operator

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crdannotator "github.com/openshift/external-secrets-operator/pkg/controller/crd_annotator"
	escontroller "github.com/openshift/external-secrets-operator/pkg/controller/external_secrets"
	esmcontroller "github.com/openshift/external-secrets-operator/pkg/controller/external_secrets_manager"
	"github.com/openshift/external-secrets-operator/pkg/webhook"
)

// webhookClientWrapper wraps client.Client to implement ctrlClient.CtrlClient
type webhookClientWrapper struct {
	c client.Client
}

// Get implements CtrlClient interface (without options)
func (w *webhookClientWrapper) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	return w.c.Get(ctx, key, obj)
}

// List implements CtrlClient interface (with options)
func (w *webhookClientWrapper) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return w.c.List(ctx, list, opts...)
}

// StatusUpdate implements CtrlClient interface
func (w *webhookClientWrapper) StatusUpdate(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return w.c.Status().Update(ctx, obj, opts...)
}

// Update implements CtrlClient interface (without options)
func (w *webhookClientWrapper) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return w.c.Update(ctx, obj, opts...)
}

// UpdateWithRetry implements CtrlClient interface
func (w *webhookClientWrapper) UpdateWithRetry(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return w.c.Update(ctx, obj, opts...)
}

// Create implements CtrlClient interface (without options)
func (w *webhookClientWrapper) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return w.c.Create(ctx, obj, opts...)
}

// Delete implements CtrlClient interface (without options)
func (w *webhookClientWrapper) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return w.c.Delete(ctx, obj, opts...)
}

// Patch implements CtrlClient interface (without options)
func (w *webhookClientWrapper) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return w.c.Patch(ctx, obj, patch, opts...)
}

// Exists implements CtrlClient interface
func (w *webhookClientWrapper) Exists(ctx context.Context, key client.ObjectKey, obj client.Object) (bool, error) {
	err := w.c.Get(ctx, key, obj)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func StartControllers(ctx context.Context, mgr ctrl.Manager) error {
	logger := ctrl.Log.WithName("setup")

	if err := esmcontroller.New(ctx, mgr).SetupWithManager(mgr); err != nil {
		logger.Error(err, "failed to set up controller with manager",
			"controller", esmcontroller.ControllerName)
		return err
	}

	externalSecretsConfig, err := escontroller.New(ctx, mgr)
	if err != nil {
		logger.Error(err, "failed to create controller", "controller", escontroller.ControllerName)
		return err
	}
	if err = externalSecretsConfig.SetupWithManager(mgr); err != nil {
		logger.Error(err, "failed to set up controller with manager",
			"controller", escontroller.ControllerName)
		return err
	}

	if externalSecretsConfig.IsCertManagerInstalled() {
		crdAnnotator, err := crdannotator.New(mgr)
		if err != nil {
			logger.Error(err, "failed to create crd annotator controller", "controller", crdannotator.ControllerName)
			return err
		}
		if err = crdAnnotator.SetupWithManager(mgr); err != nil {
			logger.Error(err, "failed to set up crd_annotator controller with manager",
				"controller", crdannotator.ControllerName)
			return err
		}
	}

	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		logger.Error(err, "failed to create uncached client")
		return err
	}
	if err = esmcontroller.CreateDefaultESMResource(ctx, uncachedClient); err != nil {
		// Log warning but don't fail startup - the controller will reconcile and create it later
		// This handles cases where CRDs are in a terminating state or temporarily unavailable
		logger.Info("could not create default externalsecretsmanagers.operator.openshift.io resource, will be created by controller reconciliation", "error", err.Error())
	}

	// Note: Cache indexes are now set up in NewCacheBuilder (before cache starts)
	// See pkg/controller/external_secrets/controller.go:setupWebhookIndexes

	// Set up webhook
	if err := setupWebhook(ctx, mgr); err != nil {
		logger.Error(err, "failed to set up webhook")
		return err
	}

	return nil
}

func setupWebhook(ctx context.Context, mgr ctrl.Manager) error {
	logger := ctrl.Log.WithName("webhook-setup")

	// Create wrapper client for webhook
	webhookClient := &webhookClientWrapper{c: mgr.GetClient()}

	// Create webhook validator
	validator := &webhook.ExternalSecretsConfigValidator{
		Client:      webhookClient,
		CacheReader: mgr.GetCache(), // Direct cache access for indexed queries!
		CacheSyncCheck: func(ctx context.Context) bool {
			// WaitForCacheSync returns true if all caches are synced
			return mgr.GetCache().WaitForCacheSync(ctx)
		},
	}

	// Register the webhook
	if err := validator.SetupWebhookWithManager(mgr); err != nil {
		return err
	}

	logger.Info("webhook successfully configured")
	return nil
}
