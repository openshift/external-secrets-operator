/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	webhook "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	// requestEnqueueLabelKey is the label key name used for filtering reconcile
	// events to include only the resources created by the controller.
	requestEnqueueLabelKey = "app"

	// requestEnqueueLabelValue is the label value used for filtering reconcile
	// events to include only the resources created by the controller.
	requestEnqueueLabelValue = "external-secrets"

	// watchResources is the list of resources that the controller watches,
	// and creates informers for.
	controllerManageResources = []client.Object{
		&certmanagerv1.Certificate{},
		&appsv1.Deployment{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&corev1.Secret{},
		&webhook.ValidatingWebhookConfiguration{},
	}
)

// ExternalSecretsReconciler reconciles a ExternalSecrets object
type ExternalSecretsReconciler struct {
	ctrlClient
	Scheme        *runtime.Scheme
	ctx           context.Context
	eventRecorder record.EventRecorder
	log           logr.Logger
}

// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecrets/finalizers,verbs=update

// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=ClusterRoleBinding;ClusterRole;RoleBinding;Role,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=ValidatingWebhookConfiguration,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=Service;ServiceAccount,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=Deployment,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=Certificate,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=Secret,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by

func New(mgr ctrl.Manager) (*ExternalSecretsReconciler, error) {
	c, err := NewClient(mgr)
	if err != nil {
		return nil, err
	}
	return &ExternalSecretsReconciler{
		ctrlClient:    c,
		ctx:           context.Background(),
		eventRecorder: mgr.GetEventRecorderFor(ControllerName),
		log:           ctrl.Log.WithName(ControllerName),
		Scheme:        mgr.GetScheme(),
	}, nil
}

func BuildCustomClient(mgr ctrl.Manager) (client.Client, error) {
	managedResourceLabelReq, _ := labels.NewRequirement(requestEnqueueLabelKey, selection.Equals, []string{requestEnqueueLabelValue})
	managedResourceLabelReqSelector := labels.NewSelector().Add(*managedResourceLabelReq)

	labelSelectors := make(map[client.Object]cache.ByObject)
	for _, res := range controllerManageResources {
		labelSelectors[res] = cache.ByObject{
			Label: managedResourceLabelReqSelector,
		}
	}
	customCacheOpts := cache.Options{
		HTTPClient:                  mgr.GetHTTPClient(),
		Scheme:                      mgr.GetScheme(),
		Mapper:                      mgr.GetRESTMapper(),
		ByObject:                    labelSelectors,
		ReaderFailOnMissingInformer: true,
	}
	customCache, err := cache.New(mgr.GetConfig(), customCacheOpts)
	if err != nil {
		return nil, err
	}

	for _, res := range controllerManageResources {
		if _, err = customCache.GetInformer(context.Background(), res); err != nil {
			return nil, err
		}
	}
	customCache.GetInformer(context.Background(), &operatorv1alpha1.ExternalSecrets{})

	err = mgr.Add(customCache)
	if err != nil {
		return nil, err
	}

	customClient, err := client.New(mgr.GetConfig(), client.Options{
		HTTPClient: mgr.GetHTTPClient(),
		Scheme:     mgr.GetScheme(),
		Mapper:     mgr.GetRESTMapper(),
		Cache: &client.CacheOptions{
			Reader: customCache,
		},
	})
	if err != nil {
		return nil, err
	}

	return customClient, nil
}

// the ExternalSecrets object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ExternalSecretsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.V(1).Info("reconciling", "request", req)

	// Fetch the external-secrets.openshift.operator.io CR
	externalsecrets := &operatorv1alpha1.ExternalSecrets{}
	if err := r.Get(ctx, req.NamespacedName, externalsecrets); err != nil {
		if errors.IsNotFound(err) {
			// NotFound errors, since they can't be fixed by an immediate
			// requeue (have to wait for a new notification), and can be processed
			// on deleted requests.
			r.log.V(1).Info("external-secrets.openshift.operator.io object not found, skipping reconciliation", "request", req)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to fetch external-secrets.openshift.operator.io %q during reconciliation: %w", req.NamespacedName, err)
	}

	if !externalsecrets.DeletionTimestamp.IsZero() {
		r.log.V(1).Info("external-secrets.openshift.operator.io is marked for deletion", "namespace", req.NamespacedName)

		if requeue, err := r.cleanUp(externalsecrets); err != nil {
			return ctrl.Result{}, fmt.Errorf("clean up failed for %q external-secrets.openshift.operator.io instance deletion: %w", req.NamespacedName, err)
		} else if requeue {
			return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
		}

		if err := r.removeFinalizer(ctx, externalsecrets, finalizer); err != nil {
			return ctrl.Result{}, err
		}

		r.log.V(1).Info("removed finalizer, cleanup complete", "request", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Set finalizers on the external-secrets.openshift.operator.io resource
	if err := r.addFinalizer(ctx, externalsecrets); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update %q external-secrets.openshift.operator.io with finalizers: %w", req.NamespacedName, err)
	}

	return r.processReconcileRequest(externalsecrets, req.NamespacedName)
}

func (r *ExternalSecretsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mapFunc := func(ctx context.Context, obj client.Object) []reconcile.Request {
		r.log.V(4).Info("received reconcile event", "object", fmt.Sprintf("%T", obj), "name", obj.GetName(), "namespace", obj.GetNamespace())
		kind := obj.GetObjectKind().GroupVersionKind().String()
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      kind + "#" + externalsecretsObjectName,
					Namespace: obj.GetNamespace(),
				},
			},
		}
	}

	controllerManagedResources := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetLabels() != nil && object.GetLabels()[requestEnqueueLabelKey] == requestEnqueueLabelValue
	})
	withIgnoreStatusUpdatePredicates := builder.WithPredicates(predicate.GenerationChangedPredicate{}, controllerManagedResources)
	controllerManagedResourcePredicates := builder.WithPredicates(controllerManagedResources)

	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ExternalSecrets{}).
		For(&operatorv1alpha1.ExternalSecrets{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(ControllerName).
		Watches(&operatorv1alpha1.ExternalSecretsOperator{}, handler.EnqueueRequestsFromMapFunc(mapFunc), withIgnoreStatusUpdatePredicates).
		Watches(&certmanagerv1.Certificate{}, handler.EnqueueRequestsFromMapFunc(mapFunc), withIgnoreStatusUpdatePredicates).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(mapFunc), withIgnoreStatusUpdatePredicates).
		Watches(&rbacv1.ClusterRole{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.ClusterRoleBinding{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.Role{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.RoleBinding{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.ServiceAccount{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&webhook.ValidatingWebhookConfiguration{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Complete(r)
}

func (r *ExternalSecretsReconciler) processReconcileRequest(externalsecrets *operatorv1alpha1.ExternalSecrets, req types.NamespacedName) (ctrl.Result, error) {
	createRecon := false
	if !containsProcessedAnnotation(externalsecrets) && reflect.DeepEqual(externalsecrets.Status, operatorv1alpha1.ExternalSecretsStatus{}) {
		r.log.V(1).Info("starting reconciliation of newly created external-secrets", "namespace", externalsecrets.GetNamespace(), "name", externalsecrets.GetName())
		createRecon = true
	}

	var errUpdate error

	err := r.reconcileExternalSecretsDeployment(externalsecrets, createRecon)
	if err != nil {
		r.log.Error(err, "failed to reconcile external-secrets deployment", "request", req)
		isFatal := IsIrrecoverableError(err)

		degradedCond := metav1.Condition{
			Type:               operatorv1alpha1.Degraded,
			ObservedGeneration: externalsecrets.GetGeneration(),
		}
		readyCond := metav1.Condition{
			Type:               operatorv1alpha1.Ready,
			ObservedGeneration: externalsecrets.GetGeneration(),
		}

		if isFatal {
			degradedCond.Status = metav1.ConditionTrue
			degradedCond.Reason = operatorv1alpha1.ReasonFailed
			degradedCond.Message = fmt.Sprintf("reconciliation failed with irrecoverable error, not retrying: %v", err)

			readyCond.Status = metav1.ConditionFalse
			readyCond.Reason = operatorv1alpha1.ReasonReady
			readyCond.Message = ""
		} else {
			degradedCond.Status = metav1.ConditionFalse
			degradedCond.Reason = operatorv1alpha1.ReasonReady
			degradedCond.Message = ""

			readyCond.Status = metav1.ConditionFalse
			readyCond.Reason = operatorv1alpha1.ReasonInProgress
			readyCond.Message = fmt.Sprintf("reconciliation failed, retrying: %v", err)
		}

		updated := false
		if apimeta.SetStatusCondition(&externalsecrets.Status.Conditions, degradedCond) {
			updated = true
		}
		if apimeta.SetStatusCondition(&externalsecrets.Status.Conditions, readyCond) {
			updated = true
		}
		if updated {
			errUpdate = r.updateCondition(externalsecrets, err)
		}

		if isFatal {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: defaultRequeueTime}, fmt.Errorf("failed to reconcile %q external-secrets deployment: %w", req, err)
	}

	// Successful reconciliation
	degradedCond := metav1.Condition{
		Type:               operatorv1alpha1.Degraded,
		Status:             metav1.ConditionFalse,
		Reason:             operatorv1alpha1.ReasonReady,
		Message:            "",
		ObservedGeneration: externalsecrets.GetGeneration(),
	}
	readyCond := metav1.Condition{
		Type:               operatorv1alpha1.Ready,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonReady,
		Message:            "reconciliation successful",
		ObservedGeneration: externalsecrets.GetGeneration(),
	}

	updated := false
	if apimeta.SetStatusCondition(&externalsecrets.Status.Conditions, degradedCond) {
		updated = true
	}
	if apimeta.SetStatusCondition(&externalsecrets.Status.Conditions, readyCond) {
		updated = true
	}
	if updated {
		errUpdate = r.updateCondition(externalsecrets, nil)
	}

	return ctrl.Result{}, errUpdate
}

// cleanUp handles deletion of external-secrets.openshift.operator.io gracefully.
func (r *ExternalSecretsReconciler) cleanUp(externalsecrets *operatorv1alpha1.ExternalSecrets) (bool, error) {
	r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeWarning, "RemoveDeployment", "%s/%s external-secrets marked for deletion, remove reference in  deployment and remove all resources created for deployment", externalsecrets.GetNamespace(), externalsecrets.GetName())
	return false, nil
}
