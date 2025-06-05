package operator

import (
	ctrl "sigs.k8s.io/controller-runtime"

	crdannotator "github.com/openshift/external-secrets-operator/pkg/controller/crd_annotator"
	externalsecretscontroller "github.com/openshift/external-secrets-operator/pkg/controller/external_secrets"
)

func StartControllers(mgr ctrl.Manager) error {
	logger := ctrl.Log.WithName("setup")

	externalsecrets, err := externalsecretscontroller.New(mgr)
	if err != nil {
		logger.Error(err, "failed to create controller", "controller", externalsecretscontroller.ControllerName)
		return err
	}
	if err = externalsecrets.SetupWithManager(mgr); err != nil {
		logger.Error(err, "failed to set up controller with manager",
			"controller", externalsecretscontroller.ControllerName)
		return err
	}

	if externalsecrets.IsCertManagerInstalled() {
		crdAnnotator, err := crdannotator.New(mgr)
		if err != nil {
			logger.Error(err, "failed to create crd annotator controller", "controller", externalsecretscontroller.ControllerName)
			return err
		}
		if err = crdAnnotator.SetupWithManager(mgr); err != nil {
			logger.Error(err, "failed to set up crd_annotator controller with manager",
				"controller", crdannotator.ControllerName)
			return err
		}
	}

	return nil
}
