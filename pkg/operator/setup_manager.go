package operator

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crdannotator "github.com/openshift/external-secrets-operator/pkg/controller/crd_annotator"
	escontroller "github.com/openshift/external-secrets-operator/pkg/controller/external_secrets"
	esmcontroller "github.com/openshift/external-secrets-operator/pkg/controller/external_secrets_manager"
)

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
		crdAnnotator, err := crdannotator.New(ctx, mgr)
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
		logger.Error(err, "failed to create default externalsecretsmanagers.operator.openshift.io resource")
		return err
	}

	return nil
}
