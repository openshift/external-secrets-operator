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

package external_secrets

import (
	"context"
	"fmt"
	"reflect"

	webhook "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	operatorclient "github.com/openshift/external-secrets-operator/pkg/controller/client"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
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
		&networkingv1.NetworkPolicy{},
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&corev1.ConfigMap{},
		&webhook.ValidatingWebhookConfiguration{},
	}
)

// Reconciler reconciles a ExternalSecretsConfig object.
type Reconciler struct {
	operatorclient.CtrlClient

	UncachedClient        operatorclient.CtrlClient
	Scheme                *runtime.Scheme
	ctx                   context.Context
	eventRecorder         record.EventRecorder
	log                   logr.Logger
	esm                   *operatorv1alpha1.ExternalSecretsManager
	optionalResourcesList map[string]struct{}
}

// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecretsconfigs,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecretsconfigs/status,verbs=get;update
// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecretsconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecretsmanagers,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch

// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=events;secrets;services;serviceaccounts,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates;clusterissuers;issuers,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch

// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=serviceaccounts/token,verbs=create
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=external-secrets.io,resources=clusterexternalsecrets;clustersecretstores;clusterpushsecrets;externalsecrets;secretstores;pushsecrets,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=external-secrets.io,resources=clusterexternalsecrets/finalizers;clustersecretstores/finalizers;externalsecrets/finalizers;pushsecrets/finalizers;secretstores/finalizers;clusterpushsecrets/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=external-secrets.io,resources=clusterexternalsecrets/status;clustersecretstores/status;externalsecrets/status;pushsecrets/status;secretstores/status;clusterpushsecrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=generators.external-secrets.io,resources=acraccesstokens;cloudsmithaccesstokens;clustergenerators;ecrauthorizationtokens;fakes;gcraccesstokens;generatorstates;sshkeys;mfas,verbs=get;list;watch;create;delete;update;patch;deletecollection
// +kubebuilder:rbac:groups=generators.external-secrets.io,resources=githubaccesstokens;grafanas;passwords;quayaccesstokens;stssessiontokens;uuids;vaultdynamicsecrets;webhooks,verbs=get;list;watch;create;delete;update;patch;deletecollection

// New is for building the reconciler instance consumed by the Reconcile method.
func New(ctx context.Context, mgr ctrl.Manager) (*Reconciler, error) {
	r := &Reconciler{
		ctx:                   ctx,
		eventRecorder:         mgr.GetEventRecorderFor(ControllerName),
		log:                   ctrl.Log.WithName(ControllerName),
		Scheme:                mgr.GetScheme(),
		esm:                   new(operatorv1alpha1.ExternalSecretsManager),
		optionalResourcesList: make(map[string]struct{}),
	}

	// Check if cert-manager is installed and register Certificate informer if present
	certManagerInstalled, err := checkAndRegisterCertificates(mgr, r)
	if err != nil {
		return nil, err
	}
	r.log.V(1).Info("Cert-manager check complete", "installed", certManagerInstalled)

	// Use the manager's client - it reads from the manager's cache
	// which is configured with label selectors via NewCacheBuilder()
	c, err := NewClient(mgr, r)
	if err != nil {
		return nil, err
	}
	r.CtrlClient = c

	// create an uncached client for the objects not managed by
	// the controller.
	uc, err := NewUncachedClient(mgr)
	if err != nil {
		return nil, err
	}
	r.UncachedClient = uc

	return r, nil
}

// NewClient returns a client that uses the manager's cache.
// The manager's cache is already configured with proper label selectors via NewCacheBuilder().
func NewClient(m manager.Manager, r *Reconciler) (operatorclient.CtrlClient, error) {
	// Use the manager's client directly - it reads from the manager's cache
	// which is now configured with the same label selectors we previously had in custom cache
	return &operatorclient.CtrlClientImpl{
		Client: m.GetClient(),
	}, nil
}

// NewUncachedClient is for creating an uncached client, and all the objects are read and written directly
// through API server.
func NewUncachedClient(m manager.Manager) (operatorclient.CtrlClient, error) {
	c, err := client.New(m.GetConfig(), client.Options{Scheme: m.GetScheme()})
	if err != nil {
		return nil, fmt.Errorf("failed to create uncached client: %w", err)
	}
	return &operatorclient.CtrlClientImpl{
		Client: c,
	}, nil
}

// NewCacheBuilder returns a cache builder function that configures the manager's cache
// with label selectors for managed resources. This eliminates the need for a separate custom cache.
func NewCacheBuilder(config *rest.Config) cache.NewCacheFunc {
	// Check if cert-manager CRD exists BEFORE creating the cache
	// This prevents cache creation failure if we try to watch a non-existent CRD
	certManagerExists, err := isCRDInstalled(config, certificateCRDName, certificateCRDGroupVersion)
	if err != nil {
		ctrl.Log.V(1).WithName("cache-setup").Error(err, "Failed to check cert-manager CRD, assuming not installed")
		certManagerExists = false
	}

	return func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
		// Build the object list with label selectors
		objectList := buildCacheObjectList(certManagerExists)

		// Configure cache options with our label-filtered resources
		opts.ByObject = objectList

		// Create and return the cache using the standard cache constructor
		return cache.New(config, opts)
	}
}

// buildCacheObjectList creates the cache configuration with label selectors
// for managed resources.
func buildCacheObjectList(includeCertManager bool) map[client.Object]cache.ByObject {
	managedResourceLabelReq, _ := labels.NewRequirement(requestEnqueueLabelKey, selection.Equals, []string{requestEnqueueLabelValue})
	managedResourceLabelReqSelector := labels.NewSelector().Add(*managedResourceLabelReq)

	objectList := make(map[client.Object]cache.ByObject)

	// Resources created by the controller - filter by app=external-secrets label
	for _, res := range controllerManagedResources {
		objectList[res] = cache.ByObject{
			Label: managedResourceLabelReqSelector,
		}
	}

	// Own CRs - no label filter needed (controller always needs to read these)
	objectList[&operatorv1alpha1.ExternalSecretsConfig{}] = cache.ByObject{}
	objectList[&operatorv1alpha1.ExternalSecretsManager{}] = cache.ByObject{}

	// Certificate objects - only include if cert-manager CRD exists
	if includeCertManager {
		objectList[&certmanagerv1.Certificate{}] = cache.ByObject{
			Label: managedResourceLabelReqSelector,
		}
	}

	return objectList
}

// checkAndRegisterCertificates checks if cert-manager CRD exists and registers Certificate informer if present.
// Returns true if Certificate CRD is installed.
func checkAndRegisterCertificates(mgr ctrl.Manager, r *Reconciler) (bool, error) {
	exist, err := isCRDInstalled(mgr.GetConfig(), certificateCRDName, certificateCRDGroupVersion)
	if err != nil {
		return false, fmt.Errorf("failed to check %s/%s CRD is installed: %w", certificateCRDGroupVersion, certificateCRDName, err)
	}

	if exist {
		r.optionalResourcesList[certificateCRDGKV] = struct{}{}

		// Get informer for Certificate - this registers it with the manager's cache
		_, err = mgr.GetCache().GetInformer(context.Background(), &certmanagerv1.Certificate{})
		if err != nil {
			return false, fmt.Errorf("failed to add Certificate informer: %w", err)
		}

		ctrl.Log.V(1).WithName("cache-setup").Info("Registered Certificate resource with manager cache")
	}

	return exist, nil
}

// SetupWithManager is for creating a controller instance with predicates and event filters.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	mapFunc := func(ctx context.Context, obj client.Object) []reconcile.Request {
		r.log.V(4).Info("received reconcile event", "object", fmt.Sprintf("%T", obj), "name", obj.GetName(), "namespace", obj.GetNamespace())

		objLabels := obj.GetLabels()
		if objLabels != nil {
			if objLabels[requestEnqueueLabelKey] == requestEnqueueLabelValue {
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Name: common.ExternalSecretsConfigObjectName,
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
		For(&operatorv1alpha1.ExternalSecretsConfig{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(ControllerName)

	for _, res := range controllerManagedResources {
		switch res {
		case &appsv1.Deployment{}:
			mgrBuilder.Watches(res, handler.EnqueueRequestsFromMapFunc(mapFunc), withIgnoreStatusUpdatePredicates)
		case &corev1.Secret{}:
			mgrBuilder.WatchesMetadata(res, handler.EnqueueRequestsFromMapFunc(mapFunc), builder.WithPredicates(predicate.LabelChangedPredicate{}))
		default: // Trusted CA ConfigMap depends on this case
			mgrBuilder.Watches(res, handler.EnqueueRequestsFromMapFunc(mapFunc), managedResourcePredicate)
		}
	}

	// Watch ExternalSecretsManager
	mgrBuilder.Watches(&operatorv1alpha1.ExternalSecretsManager{}, handler.EnqueueRequestsFromMapFunc(mapFunc), withIgnoreStatusUpdatePredicates)

	// Conditionally watch Certificate if cert-manager is installed
	// Note: Certificate is already declared in buildCacheObjectList(), this just sets up the watch
	if _, ok := r.optionalResourcesList[certificateCRDGKV]; ok {
		mgrBuilder.Watches(&certmanagerv1.Certificate{}, handler.EnqueueRequestsFromMapFunc(mapFunc), managedResourcePredicate)
	}

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
// deployment to reflect desired state configured in `externalsecretsconfigs.operator.openshift.io`.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.V(1).Info("reconciling", "request", req)

	// Fetch the externalsecretsconfigs.operator.openshift.io CR
	esc := &operatorv1alpha1.ExternalSecretsConfig{}
	if err := r.Get(ctx, req.NamespacedName, esc); err != nil {
		if errors.IsNotFound(err) {
			// NotFound errors, since they can't be fixed by an immediate
			// requeue (have to wait for a new notification), and can be processed
			// on deleted requests.
			r.log.V(1).Info("externalsecretsconfigs.operator.openshift.io object not found, skipping reconciliation", "request", req)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to fetch externalsecretsconfigs.operator.openshift.io %q during reconciliation: %w", req.NamespacedName, err)
	}

	if !esc.DeletionTimestamp.IsZero() {
		r.log.V(1).Info("externalsecretsconfigs.operator.openshift.io is marked for deletion", "name", req.NamespacedName)

		if requeue, err := r.cleanUp(esc, req); err != nil {
			return ctrl.Result{}, fmt.Errorf("clean up failed for %q externalsecretsconfigs.operator.openshift.io instance deletion: %w", req.NamespacedName, err)
		} else if requeue {
			return ctrl.Result{RequeueAfter: common.DefaultRequeueTime}, nil
		}
	}

	// Set finalizers on the externalsecretsconfigs.operator.openshift.io resource
	if err := common.AddFinalizer(ctx, esc, r.CtrlClient, finalizer); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update %q externalsecretsconfigs.operator.openshift.io with finalizers: %w", req.NamespacedName, err)
	}

	// Fetch the externalsecretsmanagers.operator.openshift.io CR
	esmNamespacedName := types.NamespacedName{
		Name: common.ExternalSecretsManagerObjectName,
	}
	if err := r.Get(ctx, esmNamespacedName, r.esm); err != nil {
		if errors.IsNotFound(err) {
			// NotFound errors, since they can't be fixed by an immediate
			// requeue (have to wait for a new notification).
			r.log.V(1).Info("externalsecretsmanagers.operator.openshift.io object not found, continuing without it")
		} else {
			return ctrl.Result{}, fmt.Errorf("failed to fetch externalsecretsmanagers.operator.openshift.io %q during reconciliation: %w", esmNamespacedName, err)
		}
	}

	return r.processReconcileRequest(esc, req.NamespacedName)
}

func (r *Reconciler) processReconcileRequest(esc *operatorv1alpha1.ExternalSecretsConfig, req types.NamespacedName) (ctrl.Result, error) {
	createRecon := false
	if !containsProcessedAnnotation(esc) && reflect.DeepEqual(esc.Status, operatorv1alpha1.ExternalSecretsConfigStatus{}) {
		r.log.V(1).Info("starting reconciliation of newly created externalsecretsconfigs.operator.openshift.io", "namespace", esc.GetNamespace(), "name", esc.GetName())
		createRecon = true
	}

	var errUpdate error = nil
	observedGeneration := esc.GetGeneration()
	err := r.reconcileExternalSecretsDeployment(esc, createRecon)
	if err != nil {
		r.log.Error(err, "failed to reconcile external-secrets deployment", "request", req)
		isFatal := common.IsIrrecoverableError(err)

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

		if apimeta.SetStatusCondition(&esc.Status.Conditions, degradedCond) ||
			apimeta.SetStatusCondition(&esc.Status.Conditions, readyCond) {
			errUpdate = r.updateCondition(esc, err)
			err = utilerrors.NewAggregate([]error{err, errUpdate})
		}

		if isFatal {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: common.DefaultRequeueTime}, fmt.Errorf("failed to reconcile %q external-secrets deployment: %w", req, err)
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

	if apimeta.SetStatusCondition(&esc.Status.Conditions, degradedCond) ||
		apimeta.SetStatusCondition(&esc.Status.Conditions, readyCond) {
		errUpdate = r.updateCondition(esc, nil)
	}

	return ctrl.Result{}, errUpdate
}

// cleanUp handles deletion of externalsecretsconfigs.operator.openshift.io gracefully.
func (r *Reconciler) cleanUp(esc *operatorv1alpha1.ExternalSecretsConfig, req ctrl.Request) (bool, error) {
	// TODO: For GA, handle cleaning up of resources created for installing external-secrets operand.
	r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "RemoveDeployment", "%s/%s externalsecretsconfigs.operator.openshift.io marked for deletion, remove reference in deployment and remove all resources created for deployment", esc.GetNamespace(), esc.GetName())

	if err := common.RemoveFinalizer(r.ctx, esc, r.CtrlClient, finalizer); err != nil {
		return true, err
	}
	r.log.V(1).Info("removed finalizer, cleanup complete", "request", req.NamespacedName)

	return false, nil
}
