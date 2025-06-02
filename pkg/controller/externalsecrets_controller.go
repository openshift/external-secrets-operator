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
	"reflect"

	"github.com/go-logr/logr"
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
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

var (
	// requestEnqueueLabelKey is the label key name used for filtering reconcile
	// events to include only the resources created by the controller.
	requestEnqueueLabelKey = "app"

	// requestEnqueueLabelValue is the label value used for filtering reconcile
	// events to include only the resources created by the controller.
	requestEnqueueLabelValue = "external-secrets"

	// controllerManagedResources is the list of resources that the controller watches,
	// and creates informers for.
	controllerManagedResources = []client.Object{
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&appsv1.Deployment{},
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&webhook.ValidatingWebhookConfiguration{},
	}
)

// ExternalSecretsReconciler reconciles a ExternalSecrets object
type ExternalSecretsReconciler struct {
	ctrlClient
	Scheme                *runtime.Scheme
	ctx                   context.Context
	eventRecorder         record.EventRecorder
	log                   logr.Logger
	esm                   *operatorv1alpha1.ExternalSecretsManager
	optionalResourcesList map[client.Object]struct{}
}

// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecretsmanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecrets/finalizers,verbs=update
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events;secrets;services;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create

// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=serviceaccounts/token,verbs=create
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=external-secrets.io,resources=clusterexternalsecrets;clustersecretstores;externalsecrets;pushsecrets;secretstores,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=external-secrets.io,resources=clusterexternalsecrets/finalizers;clustersecretstores/finalizers;externalsecrets/finalizers;pushsecrets/finalizers;secretstores/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=external-secrets.io,resources=clusterexternalsecrets/status;clustersecretstores/status;externalsecrets/status;pushsecrets/status;secretstores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=generators.external-secrets.io,resources=acraccesstokens;clustergenerators;ecrauthorizationtokens;fakes;gcraccesstokens;generatorstates,verbs=get;list;watch;create;delete;update;patch;deletecollection
// +kubebuilder:rbac:groups=generators.external-secrets.io,resources=githubaccesstokens;grafanas;passwords;quayaccesstokens;stssessiontokens;uuids;vaultdynamicsecrets;webhooks,verbs=get;list;watch;create;delete;update;patch;deletecollection

// New is for building the reconciler instance consumed by the Reconcile method.
func New(mgr ctrl.Manager) (*ExternalSecretsReconciler, error) {
	r := &ExternalSecretsReconciler{
		ctx:                   context.Background(),
		eventRecorder:         mgr.GetEventRecorderFor(ControllerName),
		log:                   ctrl.Log.WithName(ControllerName),
		Scheme:                mgr.GetScheme(),
		esm:                   new(operatorv1alpha1.ExternalSecretsManager),
		optionalResourcesList: make(map[client.Object]struct{}),
	}
	c, err := NewClient(mgr, r)
	if err != nil {
		return nil, err
	}
	r.ctrlClient = c
	return r, nil
}

// BuildCustomClient creates a custom client with a custom cache of required objects.
// The corresponding informers receive events for objects matching label criteria.
func BuildCustomClient(mgr ctrl.Manager, r *ExternalSecretsReconciler) (client.Client, error) {
	managedResourceLabelReq, _ := labels.NewRequirement(requestEnqueueLabelKey, selection.Equals, []string{requestEnqueueLabelValue})
	managedResourceLabelReqSelector := labels.NewSelector().Add(*managedResourceLabelReq)

	objectList := make(map[client.Object]cache.ByObject)
	for _, res := range controllerManagedResources {
		objectList[res] = cache.ByObject{
			Label: managedResourceLabelReqSelector,
		}
	}
	ownObject := &operatorv1alpha1.ExternalSecrets{}
	objectList[ownObject] = cache.ByObject{}
	esmObject := &operatorv1alpha1.ExternalSecretsManager{}
	objectList[esmObject] = cache.ByObject{}

	exist, err := isCRDInstalled(mgr.GetConfig(), certificateCRDName, certificateCRDGroupVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to check %s/%s CRD is installed: %w", certificateCRDGroupVersion, certificateCRDName, err)
	}
	certificateObject := &certmanagerv1.Certificate{}
	if exist {
		r.optionalResourcesList[certificateObject] = struct{}{}
		objectList[certificateObject] = cache.ByObject{
			Label: managedResourceLabelReqSelector,
		}
	}

	customCacheOpts := cache.Options{
		HTTPClient:                  mgr.GetHTTPClient(),
		Scheme:                      mgr.GetScheme(),
		Mapper:                      mgr.GetRESTMapper(),
		ByObject:                    objectList,
		ReaderFailOnMissingInformer: true,
	}
	customCache, err := cache.New(mgr.GetConfig(), customCacheOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to build custom cache: %w", err)
	}

	for _, res := range controllerManagedResources {
		if _, err = customCache.GetInformer(context.Background(), res); err != nil {
			return nil, fmt.Errorf("failed to add informer for %s resource: %w", res.GetObjectKind().GroupVersionKind().String(), err)
		}
	}
	if _, ok := r.optionalResourcesList[certificateObject]; ok {
		_, err = customCache.GetInformer(context.Background(), certificateObject)
		if err != nil {
			return nil, fmt.Errorf("failed to add informer for %s resource: %w", certificateObject.GetObjectKind().GroupVersionKind().String(), err)
		}
	}
	_, err = customCache.GetInformer(context.Background(), ownObject)
	if err != nil {
		return nil, fmt.Errorf("failed to add informer for %s resource: %w", ownObject.GetObjectKind().GroupVersionKind().String(), err)
	}
	_, err = customCache.GetInformer(context.Background(), esmObject)
	if err != nil {
		return nil, fmt.Errorf("failed to add informer for %s resource: %w", ownObject.GetObjectKind().GroupVersionKind().String(), err)
	}

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

// SetupWithManager is for creating a controller instance with predicates and event filters.
func (r *ExternalSecretsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mapFunc := func(ctx context.Context, obj client.Object) []reconcile.Request {
		r.log.V(4).Info("received reconcile event", "object", fmt.Sprintf("%T", obj), "name", obj.GetName(), "namespace", obj.GetNamespace())

		objLabels := obj.GetLabels()
		if objLabels != nil {
			if objLabels[requestEnqueueLabelKey] == requestEnqueueLabelValue {
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Name: externalsecretsObjectName,
						},
					},
				}
			}

		}
		r.log.V(4).Info("object not of interest, ignoring reconcile event", "object", fmt.Sprintf("%T", obj), "name", obj.GetName(), "namespace", obj.GetNamespace())
		return []reconcile.Request{}
	}

	// predicate function to ignore events for objects not managed by controller.
	managedResources := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetLabels() != nil && object.GetLabels()[requestEnqueueLabelKey] == requestEnqueueLabelValue
	})
	withIgnoreStatusUpdatePredicates := builder.WithPredicates(predicate.GenerationChangedPredicate{}, managedResources)
	managedResourcePredicate := builder.WithPredicates(managedResources)

	mgrBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ExternalSecrets{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(ControllerName)

	for _, res := range controllerManagedResources {
		switch res {
		case &appsv1.Deployment{}:
			mgrBuilder.Watches(res, handler.EnqueueRequestsFromMapFunc(mapFunc), withIgnoreStatusUpdatePredicates)
		case &certmanagerv1.Certificate{}:
			if _, ok := r.optionalResourcesList[res]; ok {
				mgrBuilder.Watches(res, handler.EnqueueRequestsFromMapFunc(mapFunc), managedResourcePredicate)
			}
		case &corev1.Secret{}:
			mgrBuilder.WatchesMetadata(res, handler.EnqueueRequestsFromMapFunc(mapFunc), builder.WithPredicates(predicate.LabelChangedPredicate{}))
		default:
			mgrBuilder.Watches(res, handler.EnqueueRequestsFromMapFunc(mapFunc), managedResourcePredicate)
		}
	}
	mgrBuilder.Watches(&operatorv1alpha1.ExternalSecretsManager{}, handler.EnqueueRequestsFromMapFunc(mapFunc), withIgnoreStatusUpdatePredicates)

	return mgrBuilder.Complete(r)
}

// isCRDInstalled is for checking whether a CRD with given `group/version` and `name` exists.
// TODO: Adds watches or polling to dynamically notify when a CRD gets installed.
func isCRDInstalled(config *rest.Config, name, groupVersion string) (bool, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return false, fmt.Errorf("failed to create discovery client: %w", err)
	}

	resources, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return false, fmt.Errorf("failed to discover resources list: %w", err)
	}

	for _, resource := range resources {
		if resource.GroupVersion == groupVersion {
			for _, crd := range resource.APIResources {
				if crd.Name == name {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// Reconcile is the reconciliation loop to manage the current state external-secrets
// deployment to reflect desired state configured in `externalsecrets.openshift.operator.io`.
func (r *ExternalSecretsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.V(1).Info("reconciling", "request", req)

	// Fetch the externalsecrets.openshift.operator.io CR
	externalsecrets := &operatorv1alpha1.ExternalSecrets{}
	if err := r.Get(ctx, req.NamespacedName, externalsecrets); err != nil {
		if errors.IsNotFound(err) {
			// NotFound errors, since they can't be fixed by an immediate
			// requeue (have to wait for a new notification), and can be processed
			// on deleted requests.
			r.log.V(1).Info("externalsecrets.openshift.operator.io object not found, skipping reconciliation", "request", req)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to fetch externalsecrets.openshift.operator.io %q during reconciliation: %w", req.NamespacedName, err)
	}

	if !externalsecrets.DeletionTimestamp.IsZero() {
		r.log.V(1).Info("externalsecrets.openshift.operator.io is marked for deletion", "namespace", req.NamespacedName)

		if requeue, err := r.cleanUp(externalsecrets); err != nil {
			return ctrl.Result{}, fmt.Errorf("clean up failed for %q externalsecrets.openshift.operator.io instance deletion: %w", req.NamespacedName, err)
		} else if requeue {
			return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
		}

		if err := r.removeFinalizer(ctx, externalsecrets, finalizer); err != nil {
			return ctrl.Result{}, err
		}

		r.log.V(1).Info("removed finalizer, cleanup complete", "request", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Set finalizers on the externalsecrets.openshift.operator.io resource
	if err := r.addFinalizer(ctx, externalsecrets); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update %q externalsecrets.openshift.operator.io with finalizers: %w", req.NamespacedName, err)
	}

	// Fetch the externalsecretsmanager.openshift.operator.io CR
	esmNamespacedName := types.NamespacedName{
		Name: externalsecretsManagerObjectName,
	}
	if err := r.Get(ctx, esmNamespacedName, r.esm); err != nil {
		if errors.IsNotFound(err) {
			// NotFound errors, since they can't be fixed by an immediate
			// requeue (have to wait for a new notification).
			r.log.V(1).Info("externalsecretsmanager.openshift.operator.io object not found, continuing without it")
		} else {
			return ctrl.Result{}, fmt.Errorf("failed to fetch externalsecretsmanager.openshift.operator.io %q during reconciliation: %w", esmNamespacedName, err)
		}
	}

	return r.processReconcileRequest(externalsecrets, req.NamespacedName)
}

func (r *ExternalSecretsReconciler) processReconcileRequest(externalsecrets *operatorv1alpha1.ExternalSecrets, req types.NamespacedName) (ctrl.Result, error) {
	createRecon := false
	if !containsProcessedAnnotation(externalsecrets) && reflect.DeepEqual(externalsecrets.Status, operatorv1alpha1.ExternalSecretsStatus{}) {
		r.log.V(1).Info("starting reconciliation of newly created externalsecrets.openshift.operator.io", "namespace", externalsecrets.GetNamespace(), "name", externalsecrets.GetName())
		createRecon = true
	}

	var errUpdate error = nil
	observedGeneration := externalsecrets.GetGeneration()
	err := r.reconcileExternalSecretsDeployment(externalsecrets, createRecon)
	if err != nil {
		r.log.Error(err, "failed to reconcile external-secrets deployment", "request", req)
		isFatal := IsIrrecoverableError(err)

		degradedCond := metav1.Condition{
			Type:               operatorv1alpha1.Degraded,
			ObservedGeneration: observedGeneration,
		}
		readyCond := metav1.Condition{
			Type:               operatorv1alpha1.Ready,
			ObservedGeneration: observedGeneration,
		}

		if isFatal {
			degradedCond.Status = metav1.ConditionTrue
			degradedCond.Reason = operatorv1alpha1.ReasonFailed
			degradedCond.Message = fmt.Sprintf("reconciliation failed with irrecoverable error, not retrying: %v", err)

			readyCond.Status = metav1.ConditionFalse
			readyCond.Reason = operatorv1alpha1.ReasonReady
		} else {
			degradedCond.Status = metav1.ConditionFalse
			degradedCond.Reason = operatorv1alpha1.ReasonReady

			readyCond.Status = metav1.ConditionFalse
			readyCond.Reason = operatorv1alpha1.ReasonInProgress
			readyCond.Message = fmt.Sprintf("reconciliation failed, retrying: %v", err)
		}

		if apimeta.SetStatusCondition(&externalsecrets.Status.Conditions, degradedCond) ||
			apimeta.SetStatusCondition(&externalsecrets.Status.Conditions, readyCond) {
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
		ObservedGeneration: observedGeneration,
	}
	readyCond := metav1.Condition{
		Type:               operatorv1alpha1.Ready,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonReady,
		Message:            "reconciliation successful",
		ObservedGeneration: observedGeneration,
	}

	if apimeta.SetStatusCondition(&externalsecrets.Status.Conditions, degradedCond) ||
		apimeta.SetStatusCondition(&externalsecrets.Status.Conditions, readyCond) {
		errUpdate = r.updateCondition(externalsecrets, nil)
	}

	return ctrl.Result{}, errUpdate
}

// cleanUp handles deletion of externalsecrets.openshift.operator.io gracefully.
func (r *ExternalSecretsReconciler) cleanUp(externalsecrets *operatorv1alpha1.ExternalSecrets) (bool, error) {
	// TODO: For GA, handle cleaning up of resources created for installing external-secrets operand.
	r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeWarning, "RemoveDeployment", "%s/%s externalsecrets.openshift.operator.io marked for deletion, remove reference in deployment and remove all resources created for deployment", externalsecrets.GetNamespace(), externalsecrets.GetName())
	return false, nil
}
