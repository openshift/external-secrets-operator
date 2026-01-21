package external_secrets

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

const (
	// roleBindingSubjectKind is the subject kind in the role binding object,
	// used for binding the role to preferred subject.
	roleBindingSubjectKind = "ServiceAccount"
)

// createOrApplyRBACResource is for creating all the RBAC specific resources
// required for installing external-secrets operand.
func (r *Reconciler) createOrApplyRBACResource(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string, recon bool) error {
	serviceAccountName := common.DecodeServiceAccountObjBytes(assets.MustAsset(controllerServiceAccountAssetName)).GetName()

	if err := r.createOrApplyControllerRBACResources(esc, serviceAccountName, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile controller rbac resources")
		return err
	}

	if err := r.createOrApplyCertControllerRBACResources(esc, serviceAccountName, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile cert-controller rbac resources")
		return err
	}

	return nil
}

// createOrApplyControllerRBACResources is for creating all RBAC resources required by
// the main external-secrets operand controller.
func (r *Reconciler) createOrApplyControllerRBACResources(esc *operatorv1alpha1.ExternalSecretsConfig, serviceAccountName string, resourceLabels map[string]string, recon bool) error {
	for _, asset := range []string{
		controllerClusterRoleAssetName,
		controllerClusterRoleEditAssetName,
		controllerClusterRoleServiceBindingsAssetName,
		controllerClusterRoleViewAssetName,
	} {
		clusterRoleObj := r.getClusterRoleObject(asset, resourceLabels)
		if err := r.createOrApplyClusterRole(esc, clusterRoleObj, recon); err != nil {
			r.log.Error(err, "failed to reconcile controller clusterrole resources")
			return err
		}
	}

	clusterRoleName := common.DecodeClusterRoleObjBytes(assets.MustAsset(controllerClusterRoleAssetName)).GetName()
	clusterRoleBindingObj := r.getClusterRoleBindingObject(esc, controllerClusterRoleBindingAssetName, clusterRoleName, serviceAccountName, resourceLabels)
	if err := r.createOrApplyClusterRoleBinding(esc, clusterRoleBindingObj, recon); err != nil {
		r.log.Error(err, "failed to reconcile controller clusterrolebinding resources")
		return err
	}

	roleObj := r.getRoleObject(esc, controllerRoleLeaderElectionAssetName, resourceLabels)
	if err := r.createOrApplyRole(esc, roleObj, recon); err != nil {
		r.log.Error(err, "failed to reconcile controller role resources")
		return err
	}

	roleBindingObj := r.getRoleBindingObject(esc, controllerRoleBindingLeaderElectionAssetName, roleObj.GetName(), serviceAccountName, resourceLabels)
	if err := r.createOrApplyRoleBinding(esc, roleBindingObj, recon); err != nil {
		r.log.Error(err, "failed to reconcile controller rolebinding resources")
		return err
	}

	return nil
}

// createOrApplyCertControllerRBACResources is for creating all RBAC resources required by
// the main external-secrets operand cert-controller.
func (r *Reconciler) createOrApplyCertControllerRBACResources(esc *operatorv1alpha1.ExternalSecretsConfig, serviceAccountName string, resourceLabels map[string]string, recon bool) error {
	if isCertManagerConfigEnabled(esc) {
		r.log.V(4).Info("skipping cert-controller rbac resources reconciliation, as cert-manager config is enabled")
		return nil
	}

	clusterRoleObj := r.getClusterRoleObject(certControllerClusterRoleAssetName, resourceLabels)
	if err := r.createOrApplyClusterRole(esc, clusterRoleObj, recon); err != nil {
		r.log.Error(err, "failed to reconcile cert-controller clusterrole resources")
		return err
	}

	clusterRoleBindingObj := r.getClusterRoleBindingObject(esc, certControllerClusterRoleBindingAssetName, clusterRoleObj.GetName(), serviceAccountName, resourceLabels)
	if err := r.createOrApplyClusterRoleBinding(esc, clusterRoleBindingObj, recon); err != nil {
		r.log.Error(err, "failed to reconcile cert-controller clusterrolebinding resources")
		return err
	}

	return nil
}

// createOrApplyClusterRole creates or updates given ClusterRole object.
func (r *Reconciler) createOrApplyClusterRole(esc *operatorv1alpha1.ExternalSecretsConfig, obj *rbacv1.ClusterRole, recon bool) error {
	var (
		exist           bool
		err             error
		clusterRoleName = obj.GetName()
		fetched         = &rbacv1.ClusterRole{}
	)

	exist, err = r.Exists(r.ctx, client.ObjectKeyFromObject(obj), fetched)
	if err != nil {
		return common.FromClientError(err, "failed to check %s clusterrole resource already exists", clusterRoleName)
	}

	if exist && recon {
		r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s clusterrole resource already exists, maybe from previous installation", clusterRoleName)
	}
	if exist && common.HasObjectChanged(obj, fetched) {
		r.log.V(1).Info("clusterrole has been modified, updating to desired state", "name", clusterRoleName)
		if err := r.UpdateWithRetry(r.ctx, obj); err != nil {
			return common.FromClientError(err, "failed to update %s clusterrole resource", clusterRoleName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "clusterrole resource %s reconciled back to desired state", clusterRoleName)
	} else {
		r.log.V(4).Info("clusterrole resource already exists and is in expected state", "name", clusterRoleName)
	}
	if !exist {
		if err := r.Create(r.ctx, obj); err != nil {
			return common.FromClientError(err, "failed to create %s clusterrole resource", clusterRoleName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "clusterrole resource %s created", clusterRoleName)
	}

	return nil
}

// getClusterRoleObject is for obtaining the content of given ClusterRole static asset, and
// then updating it with desired values.
func (r *Reconciler) getClusterRoleObject(assetName string, resourceLabels map[string]string) *rbacv1.ClusterRole {
	clusterRole := common.DecodeClusterRoleObjBytes(assets.MustAsset(assetName))
	common.UpdateResourceLabels(clusterRole, resourceLabels)
	return clusterRole
}

// createOrApplyClusterRoleBinding creates or updates given ClusterRoleBinding object.
func (r *Reconciler) createOrApplyClusterRoleBinding(esc *operatorv1alpha1.ExternalSecretsConfig, obj *rbacv1.ClusterRoleBinding, recon bool) error {
	var (
		exist                  bool
		err                    error
		clusterRoleBindingName = obj.GetName()
		fetched                = &rbacv1.ClusterRoleBinding{}
	)
	r.log.V(4).Info("reconciling clusterrolebinding resource", "name", clusterRoleBindingName)
	exist, err = r.Exists(r.ctx, client.ObjectKeyFromObject(obj), fetched)
	if err != nil {
		return common.FromClientError(err, "failed to check %s clusterrolebinding resource already exists", clusterRoleBindingName)
	}

	if exist && recon {
		r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s clusterrolebinding resource already exists, maybe from previous installation", clusterRoleBindingName)
	}
	if exist && common.HasObjectChanged(obj, fetched) {
		r.log.V(1).Info("clusterrolebinding has been modified, updating to desired state", "name", clusterRoleBindingName)
		if err := r.UpdateWithRetry(r.ctx, obj); err != nil {
			return common.FromClientError(err, "failed to update %s clusterrolebinding resource", clusterRoleBindingName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "clusterrolebinding resource %s reconciled back to desired state", clusterRoleBindingName)
	} else {
		r.log.V(4).Info("clusterrolebinding resource already exists and is in expected state", "name", clusterRoleBindingName)
	}
	if !exist {
		if err := r.Create(r.ctx, obj); err != nil {
			return common.FromClientError(err, "failed to create %s clusterrolebinding resource", clusterRoleBindingName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "clusterrolebinding resource %s created", clusterRoleBindingName)
	}

	return nil
}

// getClusterRoleBindingObject is for obtaining the content of given ClusterRoleBinding static asset, and
// then updating it with desired values.
func (r *Reconciler) getClusterRoleBindingObject(esc *operatorv1alpha1.ExternalSecretsConfig, assetName, clusterRoleName, serviceAccountName string, resourceLabels map[string]string) *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := common.DecodeClusterRoleBindingObjBytes(assets.MustAsset(assetName))
	clusterRoleBinding.RoleRef.Name = clusterRoleName
	common.UpdateResourceLabels(clusterRoleBinding, resourceLabels)
	updateServiceAccountNamespaceInRBACBindingObject[*rbacv1.ClusterRoleBinding](clusterRoleBinding, serviceAccountName, getNamespace(esc))
	return clusterRoleBinding
}

// createOrApplyRole creates or updates given Role object.
func (r *Reconciler) createOrApplyRole(esc *operatorv1alpha1.ExternalSecretsConfig, obj *rbacv1.Role, recon bool) error {
	roleName := fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
	r.log.V(4).Info("reconciling role resource", "name", roleName)
	fetched := &rbacv1.Role{}
	exist, err := r.Exists(r.ctx, client.ObjectKeyFromObject(obj), fetched)
	if err != nil {
		return common.FromClientError(err, "failed to check %s role resource already exists", roleName)
	}

	if exist && recon {
		r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s role resource already exists, maybe from previous installation", roleName)
	}
	if exist && common.HasObjectChanged(obj, fetched) {
		r.log.V(1).Info("role has been modified, updating to desired state", "name", roleName)
		if err := r.UpdateWithRetry(r.ctx, obj); err != nil {
			return common.FromClientError(err, "failed to update %s role resource", roleName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "role resource %s reconciled back to desired state", roleName)
	} else {
		r.log.V(4).Info("role resource already exists and is in expected state", "name", roleName)
	}
	if !exist {
		if err := r.Create(r.ctx, obj); err != nil {
			return common.FromClientError(err, "failed to create %s role resource", roleName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "role resource %s created", roleName)
	}

	return nil
}

// getRoleObject is for obtaining the content of given Role static asset, and
// then updating it with desired values.
func (r *Reconciler) getRoleObject(esc *operatorv1alpha1.ExternalSecretsConfig, assetName string, resourceLabels map[string]string) *rbacv1.Role {
	role := common.DecodeRoleObjBytes(assets.MustAsset(assetName))
	updateNamespace(role, esc)
	common.UpdateResourceLabels(role, resourceLabels)
	return role
}

// createOrApplyRoleBinding creates or updates given RoleBinding object.
func (r *Reconciler) createOrApplyRoleBinding(esc *operatorv1alpha1.ExternalSecretsConfig, obj *rbacv1.RoleBinding, recon bool) error {
	roleBindingName := fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
	r.log.V(4).Info("reconciling rolebinding resource", "name", roleBindingName)
	fetched := &rbacv1.RoleBinding{}
	exist, err := r.Exists(r.ctx, client.ObjectKeyFromObject(obj), fetched)
	if err != nil {
		return common.FromClientError(err, "failed to check %s rolebinding resource already exists", roleBindingName)
	}

	if exist && recon {
		r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s rolebinding resource already exists, maybe from previous installation", roleBindingName)
	}
	if exist && common.HasObjectChanged(obj, fetched) {
		r.log.V(1).Info("rolebinding has been modified, updating to desired state", "name", roleBindingName)
		if err := r.UpdateWithRetry(r.ctx, obj); err != nil {
			return common.FromClientError(err, "failed to update %s rolebinding resource", roleBindingName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "rolebinding resource %s reconciled back to desired state", roleBindingName)
	} else {
		r.log.V(4).Info("rolebinding resource already exists and is in expected state", "name", roleBindingName)
	}
	if !exist {
		if err := r.Create(r.ctx, obj); err != nil {
			return common.FromClientError(err, "failed to create %s rolebinding resource", roleBindingName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "rolebinding resource %s created", roleBindingName)
	}

	return nil
}

// getRoleBindingObject is for obtaining the content of given RoleBinding static asset, and
// then updating it with desired values.
func (r *Reconciler) getRoleBindingObject(esc *operatorv1alpha1.ExternalSecretsConfig, assetName, roleName, serviceAccountName string, resourceLabels map[string]string) *rbacv1.RoleBinding {
	roleBinding := common.DecodeRoleBindingObjBytes(assets.MustAsset(assetName))
	roleBinding.RoleRef.Name = roleName
	updateNamespace(roleBinding, esc)
	common.UpdateResourceLabels(roleBinding, resourceLabels)
	updateServiceAccountNamespaceInRBACBindingObject[*rbacv1.RoleBinding](roleBinding, serviceAccountName, roleBinding.GetNamespace())
	return roleBinding
}

// updateServiceAccountNamespaceInRBACBindingObject is for updating the ServiceAccount namespace in
// the ClusterRoleBinding and RoleBinding objects.
func updateServiceAccountNamespaceInRBACBindingObject[Object *rbacv1.RoleBinding | *rbacv1.ClusterRoleBinding](obj Object, serviceAccount, newNamespace string) {
	var subjects *[]rbacv1.Subject
	switch o := any(obj).(type) {
	case *rbacv1.ClusterRoleBinding:
		subjects = &o.Subjects
	case *rbacv1.RoleBinding:
		subjects = &o.Subjects
	}
	for i := range *subjects {
		if (*subjects)[i].Kind == roleBindingSubjectKind && (*subjects)[i].Name == serviceAccount {
			(*subjects)[i].Namespace = newNamespace
		}
	}
}
