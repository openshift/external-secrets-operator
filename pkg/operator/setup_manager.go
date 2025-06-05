package operator

import (
	ctrl "sigs.k8s.io/controller-runtime"

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

	return nil
}
