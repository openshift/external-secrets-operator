package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

const (
	// roleBindingSubjectKind is the subject kind in the role binding object,
	// used for binding the role to preferred subject.
	roleBindingSubjectKind = "ServiceAccount"
)

// createOrApplyRBACResource is for creating all the RBAC specific resources
// required for installing external-secrets operand.
func (r *ExternalSecretsReconciler) createOrApplyRBACResource(es *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string, recon bool) error {
	serviceAccountName := decodeServiceAccountObjBytes(assets.MustAsset(controllerServiceAccountAssetName)).GetName()

	if err := r.createOrApplyControllerRBACResources(es, serviceAccountName, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile controller rbac resources")
		return err
	}

	if err := r.createOrApplyCertControllerRBACResources(es, serviceAccountName, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile cert-controller rbac resources")
		return err
	}

	return nil
}

// createOrApplyControllerRBACResources is for creating all RBAC resources required by
// the main external-secrets operand controller.
func (r *ExternalSecretsReconciler) createOrApplyControllerRBACResources(es *operatorv1alpha1.ExternalSecrets, serviceAccountName string, resourceLabels map[string]string, recon bool) error {
	for _, asset := range []string{
		controllerClusterRoleAssetName,
		controllerClusterRoleEditAssetName,
		controllerClusterRoleServiceBindingsAssetName,
		controllerClusterRoleViewAssetName,
	} {
		clusterRoleObj := r.getClusterRoleObject(asset, resourceLabels)
		if err := r.createOrApplyClusterRole(es, clusterRoleObj, recon); err != nil {
			r.log.Error(err, "failed to reconcile controller clusterrole resources")
			return err
		}
	}

	clusterRoleName := decodeClusterRoleObjBytes(assets.MustAsset(controllerClusterRoleAssetName)).GetName()
	clusterRoleBindingObj := r.getClusterRoleBindingObject(es, controllerClusterRoleBindingAssetName, clusterRoleName, serviceAccountName, resourceLabels)
	if err := r.createOrApplyClusterRoleBinding(es, clusterRoleBindingObj, recon); err != nil {
		r.log.Error(err, "failed to reconcile controller clusterrolebinding resources")
		return err
	}

	roleObj := r.getRoleObject(es, controllerRoleLeaderElectionAssetName, resourceLabels)
	if err := r.createOrApplyRole(es, roleObj, recon); err != nil {
		r.log.Error(err, "failed to reconcile controller role resources")
		return err
	}

	roleBindingObj := r.getRoleBindingObject(es, controllerRoleBindingLeaderElectionAssetName, roleObj.GetName(), serviceAccountName, resourceLabels)
	if err := r.createOrApplyRoleBinding(es, roleBindingObj, recon); err != nil {
		r.log.Error(err, "failed to reconcile controller rolebinding resources")
		return err
	}

	return nil
}

// createOrApplyCertControllerRBACResources is for creating all RBAC resources required by
// the main external-secrets operand cert-controller.
func (r *ExternalSecretsReconciler) createOrApplyCertControllerRBACResources(es *operatorv1alpha1.ExternalSecrets, serviceAccountName string, resourceLabels map[string]string, recon bool) error {
	if isCertManagerConfigEnabled(es) {
		r.log.V(4).Info("skipping cert-controller rbac resources reconciliation, as cert-manager config is enabled")
		return nil
	}

	clusterRoleObj := r.getClusterRoleObject(certControllerClusterRoleAssetName, resourceLabels)
	if err := r.createOrApplyClusterRole(es, clusterRoleObj, recon); err != nil {
		r.log.Error(err, "failed to reconcile cert-controller clusterrole resources")
		return err
	}

	clusterRoleBindingObj := r.getClusterRoleBindingObject(es, certControllerClusterRoleBindingAssetName, clusterRoleObj.GetName(), serviceAccountName, resourceLabels)
	if err := r.createOrApplyClusterRoleBinding(es, clusterRoleBindingObj, recon); err != nil {
		r.log.Error(err, "failed to reconcile cert-controller clusterrolebinding resources")
		return err
	}

	return nil
}

// createOrApplyClusterRole creates or updates given ClusterRole object.
func (r *ExternalSecretsReconciler) createOrApplyClusterRole(es *operatorv1alpha1.ExternalSecrets, obj *rbacv1.ClusterRole, recon bool) error {
	var (
		exist           bool
		err             error
		key             types.NamespacedName
		clusterRoleName = obj.GetName()
		fetched         = &rbacv1.ClusterRole{}
	)

	key = types.NamespacedName{
		Name: clusterRoleName,
	}
	exist, err = r.Exists(r.ctx, key, fetched)
	if err != nil {
		return FromClientError(err, "failed to check %s clusterrole resource already exists", clusterRoleName)
	}

	if exist && recon {
		r.eventRecorder.Eventf(es, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s clusterrole resource already exists, maybe from previous installation", clusterRoleName)
	}
	if exist && hasObjectChanged(obj, fetched) {
		r.log.V(1).Info("clusterrole has been modified, updating to desired state", "name", clusterRoleName)
		if err := r.UpdateWithRetry(r.ctx, obj); err != nil {
			return FromClientError(err, "failed to update %s clusterrole resource", clusterRoleName)
		}
		r.eventRecorder.Eventf(es, corev1.EventTypeNormal, "Reconciled", "clusterrole resource %s reconciled back to desired state", clusterRoleName)
	} else {
		r.log.V(4).Info("clusterrole resource already exists and is in expected state", "name", clusterRoleName)
	}
	if !exist {
		if err := r.Create(r.ctx, obj); err != nil {
			return FromClientError(err, "failed to create %s clusterrole resource", clusterRoleName)
		}
		r.eventRecorder.Eventf(es, corev1.EventTypeNormal, "Reconciled", "clusterrole resource %s created", clusterRoleName)
	}

	return nil
}

// getClusterRoleObject is for obtaining the content of given ClusterRole static asset, and
// then updating it with desired values.
func (r *ExternalSecretsReconciler) getClusterRoleObject(assetName string, resourceLabels map[string]string) *rbacv1.ClusterRole {
	clusterRole := decodeClusterRoleObjBytes(assets.MustAsset(assetName))
	updateResourceLabels(clusterRole, resourceLabels)
	return clusterRole
}

// createOrApplyClusterRoleBinding creates or updates given ClusterRoleBinding object.
func (r *ExternalSecretsReconciler) createOrApplyClusterRoleBinding(es *operatorv1alpha1.ExternalSecrets, obj *rbacv1.ClusterRoleBinding, recon bool) error {
	var (
		exist                  bool
		err                    error
		key                    types.NamespacedName
		clusterRoleBindingName = obj.GetName()
		fetched                = &rbacv1.ClusterRoleBinding{}
	)
	r.log.V(4).Info("reconciling clusterrolebinding resource", "name", clusterRoleBindingName)
	key = types.NamespacedName{
		Name: clusterRoleBindingName,
	}
	exist, err = r.Exists(r.ctx, key, fetched)
	if err != nil {
		return FromClientError(err, "failed to check %s clusterrolebinding resource already exists", clusterRoleBindingName)
	}

	if exist && recon {
		r.eventRecorder.Eventf(es, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s clusterrolebinding resource already exists, maybe from previous installation", clusterRoleBindingName)
	}
	if exist && hasObjectChanged(obj, fetched) {
		r.log.V(1).Info("clusterrolebinding has been modified, updating to desired state", "name", clusterRoleBindingName)
		if err := r.UpdateWithRetry(r.ctx, obj); err != nil {
			return FromClientError(err, "failed to update %s clusterrolebinding resource", clusterRoleBindingName)
		}
		r.eventRecorder.Eventf(es, corev1.EventTypeNormal, "Reconciled", "clusterrolebinding resource %s reconciled back to desired state", clusterRoleBindingName)
	} else {
		r.log.V(4).Info("clusterrolebinding resource already exists and is in expected state", "name", clusterRoleBindingName)
	}
	if !exist {
		if err := r.Create(r.ctx, obj); err != nil {
			return FromClientError(err, "failed to create %s clusterrolebinding resource", clusterRoleBindingName)
		}
		r.eventRecorder.Eventf(es, corev1.EventTypeNormal, "Reconciled", "clusterrolebinding resource %s created", clusterRoleBindingName)
	}

	return nil
}

// getClusterRoleBindingObject is for obtaining the content of given ClusterRoleBinding static asset, and
// then updating it with desired values.
func (r *ExternalSecretsReconciler) getClusterRoleBindingObject(es *operatorv1alpha1.ExternalSecrets, assetName, clusterRoleName, serviceAccountName string, resourceLabels map[string]string) *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := decodeClusterRoleBindingObjBytes(assets.MustAsset(assetName))
	clusterRoleBinding.RoleRef.Name = clusterRoleName
	updateResourceLabels(clusterRoleBinding, resourceLabels)
	updateServiceAccountNamespaceInRBACBindingObject[*rbacv1.ClusterRoleBinding](clusterRoleBinding, serviceAccountName, getNamespace(es))
	return clusterRoleBinding
}

// createOrApplyRole creates or updates given Role object.
func (r *ExternalSecretsReconciler) createOrApplyRole(es *operatorv1alpha1.ExternalSecrets, obj *rbacv1.Role, recon bool) error {
	roleName := fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
	r.log.V(4).Info("reconciling role resource", "name", roleName)
	fetched := &rbacv1.Role{}
	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
	exist, err := r.Exists(r.ctx, key, fetched)
	if err != nil {
		return FromClientError(err, "failed to check %s role resource already exists", roleName)
	}

	if exist && recon {
		r.eventRecorder.Eventf(es, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s role resource already exists, maybe from previous installation", roleName)
	}
	if exist && hasObjectChanged(obj, fetched) {
		r.log.V(1).Info("role has been modified, updating to desired state", "name", roleName)
		if err := r.UpdateWithRetry(r.ctx, obj); err != nil {
			return FromClientError(err, "failed to update %s role resource", roleName)
		}
		r.eventRecorder.Eventf(es, corev1.EventTypeNormal, "Reconciled", "role resource %s reconciled back to desired state", roleName)
	} else {
		r.log.V(4).Info("role resource already exists and is in expected state", "name", roleName)
	}
	if !exist {
		if err := r.Create(r.ctx, obj); err != nil {
			return FromClientError(err, "failed to create %s role resource", roleName)
		}
		r.eventRecorder.Eventf(es, corev1.EventTypeNormal, "Reconciled", "role resource %s created", roleName)
	}

	return nil
}

// getRoleObject is for obtaining the content of given Role static asset, and
// then updating it with desired values.
func (r *ExternalSecretsReconciler) getRoleObject(es *operatorv1alpha1.ExternalSecrets, assetName string, resourceLabels map[string]string) *rbacv1.Role {
	role := decodeRoleObjBytes(assets.MustAsset(assetName))
	updateNamespace(role, es)
	updateResourceLabels(role, resourceLabels)
	return role
}

// createOrApplyRoleBinding creates or updates given RoleBinding object.
func (r *ExternalSecretsReconciler) createOrApplyRoleBinding(es *operatorv1alpha1.ExternalSecrets, obj *rbacv1.RoleBinding, recon bool) error {
	roleBindingName := fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
	r.log.V(4).Info("reconciling rolebinding resource", "name", roleBindingName)
	fetched := &rbacv1.RoleBinding{}
	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
	exist, err := r.Exists(r.ctx, key, fetched)
	if err != nil {
		return FromClientError(err, "failed to check %s rolebinding resource already exists", roleBindingName)
	}

	if exist && recon {
		r.eventRecorder.Eventf(es, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s rolebinding resource already exists, maybe from previous installation", roleBindingName)
	}
	if exist && hasObjectChanged(obj, fetched) {
		r.log.V(1).Info("rolebinding has been modified, updating to desired state", "name", roleBindingName)
		if err := r.UpdateWithRetry(r.ctx, obj); err != nil {
			return FromClientError(err, "failed to update %s rolebinding resource", roleBindingName)
		}
		r.eventRecorder.Eventf(es, corev1.EventTypeNormal, "Reconciled", "rolebinding resource %s reconciled back to desired state", roleBindingName)
	} else {
		r.log.V(4).Info("rolebinding resource already exists and is in expected state", "name", roleBindingName)

	}
	if !exist {
		if err := r.Create(r.ctx, obj); err != nil {
			return FromClientError(err, "failed to create %s rolebinding resource", roleBindingName)
		}
		r.eventRecorder.Eventf(es, corev1.EventTypeNormal, "Reconciled", "rolebinding resource %s created", roleBindingName)
	}

	return nil
}

// getRoleBindingObject is for obtaining the content of given RoleBinding static asset, and
// then updating it with desired values.
func (r *ExternalSecretsReconciler) getRoleBindingObject(es *operatorv1alpha1.ExternalSecrets, assetName, roleName, serviceAccountName string, resourceLabels map[string]string) *rbacv1.RoleBinding {
	roleBinding := decodeRoleBindingObjBytes(assets.MustAsset(assetName))
	roleBinding.RoleRef.Name = roleName
	updateNamespace(roleBinding, es)
	updateResourceLabels(roleBinding, resourceLabels)
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
