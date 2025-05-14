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
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

var (
	// requestEnqueueLabelKey is the label key name used for filtering reconcile
	// events to include only the resources created by the controller.
	requestEnqueueLabelKey = "app"

	// requestEnqueueLabelValue is the label value used for filtering reconcile
	// events to include only the resources created by the controller.
	requestEnqueueLabelValue = "external-secrets"
)

// ExternalSecretsReconciler reconciles a ExternalSecrets object
type ExternalSecretsReconciler struct {
	ctrlClient
	scheme        *runtime.Scheme
	ctx           context.Context
	eventRecorder record.EventRecorder
	log           logr.Logger
}

// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecrets/finalizers,verbs=update

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
		scheme:        mgr.GetScheme(),
	}, nil
}

func BuildCustomClient(mgr ctrl.Manager) (client.Client, error) {
	managedResourceLabelReq, _ := labels.NewRequirement(requestEnqueueLabelKey, selection.Equals, []string{requestEnqueueLabelValue})
	managedResourceLabelReqSelector := labels.NewSelector().Add(*managedResourceLabelReq)

	customCacheOpts := cache.Options{
		HTTPClient: mgr.GetHTTPClient(),
		Scheme:     mgr.GetScheme(),
		Mapper:     mgr.GetRESTMapper(),
		ByObject: map[client.Object]cache.ByObject{
			&certmanagerv1.Certificate{}: {
				Label: managedResourceLabelReqSelector,
			},
			&appsv1.Deployment{}: {
				Label: managedResourceLabelReqSelector,
			},
			&rbacv1.ClusterRole{}: {
				Label: managedResourceLabelReqSelector,
			},
			&rbacv1.ClusterRoleBinding{}: {
				Label: managedResourceLabelReqSelector,
			},
			&rbacv1.Role{}: {
				Label: managedResourceLabelReqSelector,
			},
			&rbacv1.RoleBinding{}: {
				Label: managedResourceLabelReqSelector,
			},
			&corev1.Service{}: {
				Label: managedResourceLabelReqSelector,
			},
			&corev1.ServiceAccount{}: {
				Label: managedResourceLabelReqSelector,
			},
			&corev1.Secret{}: {
				Label: managedResourceLabelReqSelector,
			},
		},
		ReaderFailOnMissingInformer: true,
	}
	customCache, err := cache.New(mgr.GetConfig(), customCacheOpts)
	if err != nil {
		return nil, err
	}
	if _, err = customCache.GetInformer(context.Background(), &operatorv1alpha1.ExternalSecretsOperator{}); err != nil {
		return nil, err
	}
	if _, err = customCache.GetInformer(context.Background(), &certmanagerv1.Certificate{}); err != nil {
		return nil, err
	}
	if _, err = customCache.GetInformer(context.Background(), &appsv1.Deployment{}); err != nil {
		return nil, err
	}
	if _, err = customCache.GetInformer(context.Background(), &rbacv1.ClusterRole{}); err != nil {
		return nil, err
	}
	if _, err = customCache.GetInformer(context.Background(), &rbacv1.ClusterRoleBinding{}); err != nil {
		return nil, err
	}
	if _, err = customCache.GetInformer(context.Background(), &rbacv1.Role{}); err != nil {
		return nil, err
	}
	if _, err = customCache.GetInformer(context.Background(), &rbacv1.RoleBinding{}); err != nil {
		return nil, err
	}
	if _, err = customCache.GetInformer(context.Background(), &corev1.Service{}); err != nil {
		return nil, err
	}
	if _, err = customCache.GetInformer(context.Background(), &corev1.ServiceAccount{}); err != nil {
		return nil, err
	}
	if _, err = customCache.GetInformer(context.Background(), &corev1.Secret{}); err != nil {
		return nil, err
	}
	if _, err = customCache.GetInformer(context.Background(), &corev1.ConfigMap{}); err != nil {
		return nil, err
	}
	//if _, err = customCache.GetInformer(context.Background(), &certmanagerv1.Issuer{}); err != nil {
	//	return nil, err
	//}
	//if _, err = customCache.GetInformer(context.Background(), &certmanagerv1.ClusterIssuer{}); err != nil {
	//	return nil, err
	//}

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
	externalsecrets := &operatorv1alpha1.ExternalSecretsOperator{}
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

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalSecretsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mapFunc := func(ctx context.Context, obj client.Object) []reconcile.Request {
		r.log.V(4).Info("received reconcile event", "object", fmt.Sprintf("%T", obj), "name", obj.GetName(), "namespace", obj.GetNamespace())

		objLabels := obj.GetLabels()
		if objLabels != nil {
			// will look for custom label set on objects not created in external-secrets namespace, and if it exists,
			// namespace in the reconcile request will be set same, else since label check matches is an object
			// created by controller, and we safely assume, it's in the external-secrets namespace.
			namespace := objLabels[externalsecretsNamespaceMappingLabelName]
			if namespace == "" {
				namespace = obj.GetNamespace()
			}

			labelOk := func() bool {
				if objLabels[requestEnqueueLabelKey] == requestEnqueueLabelValue {
					return true
				}
				value := objLabels[externalsecretsResourceWatchLabelName]
				if value == "" {
					return false
				}
				key := strings.Split(value, "_")
				if len(key) != 2 {
					r.log.Error(fmt.Errorf("invalid label format"), "%s label value(%s) not in expected format on %s resource", externalsecretsResourceWatchLabelName, value, obj.GetName())
					return false
				}
				namespace = key[0]
				return true
			}

			if labelOk() && namespace != "" {
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Name:      externalsecretsObjectName,
							Namespace: namespace,
						},
					},
				}
			}
		}

		r.log.V(4).Info("object not of interest, ignoring reconcile event", "object", fmt.Sprintf("%T", obj), "name", obj.GetName(), "namespace", obj.GetNamespace())
		return []reconcile.Request{}
	}

	// predicate function to ignore events for objects not managed by controller.
	controllerManagedResources := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetLabels() != nil && object.GetLabels()[requestEnqueueLabelKey] == requestEnqueueLabelValue
	})

	// predicate function to filter events for objects which controller is interested in, but
	// not managed or created by controller.
	controllerWatchResources := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetLabels() != nil && object.GetLabels()[externalsecretsResourceWatchLabelName] != ""
	})

	withIgnoreStatusUpdatePredicates := builder.WithPredicates(predicate.GenerationChangedPredicate{}, controllerManagedResources)
	controllerWatchResourcePredicates := builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}, controllerWatchResources)
	controllerManagedResourcePredicates := builder.WithPredicates(controllerManagedResources)

	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ExternalSecretsOperator{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(ControllerName).
		Watches(&certmanagerv1.Certificate{}, handler.EnqueueRequestsFromMapFunc(mapFunc), withIgnoreStatusUpdatePredicates).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(mapFunc), withIgnoreStatusUpdatePredicates).
		Watches(&rbacv1.ClusterRole{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.ClusterRoleBinding{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.Role{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.RoleBinding{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.ServiceAccount{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		WatchesMetadata(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerWatchResourcePredicates).
		Complete(r)
}

func (r *ExternalSecretsReconciler) processReconcileRequest(externalsecrets *operatorv1alpha1.ExternalSecretsOperator, req types.NamespacedName) (ctrl.Result, error) {
	createRecon := false
	if !containsProcessedAnnotation(externalsecrets) && reflect.DeepEqual(externalsecrets.Status, operatorv1alpha1.ExternalSecretsStatus{}) {
		r.log.V(1).Info("starting reconciliation of newly created external-secrets", "namespace", externalsecrets.GetNamespace(), "name", externalsecrets.GetName())
		createRecon = true
	}

	//if err := r.disallowMultipleIstioCSRInstances(externalsecrets); err != nil {
	//	if IsMultipleInstanceError(err) {
	//		r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeWarning, "MultiIstioCSRInstance", "creation of multiple istiocsr instances is not supported, will not be processed")
	//		err = nil
	//	}
	//	return ctrl.Result{}, err
	//}

	var errUpdate error = nil
	if err := r.reconcileExternalSecretsDeployment(externalsecrets, createRecon); err != nil {
		r.log.Error(err, "failed to reconcile external-secrets deployment", "request", req)
		if IsIrrecoverableError(err) {
			if externalsecrets.Status.SetCondition(operatorv1alpha1.Degraded, metav1.ConditionTrue, operatorv1alpha1.ReasonFailed, fmt.Sprintf("reconciliation failed with irrecoverable eror not retrying: %v", err)) ||
				externalsecrets.Status.SetCondition(operatorv1alpha1.Ready, metav1.ConditionFalse, operatorv1alpha1.ReasonReady, "") {
				errUpdate = r.updateCondition(externalsecrets, nil)
			}
			return ctrl.Result{}, errUpdate
		} else {
			if externalsecrets.Status.SetCondition(operatorv1alpha1.Degraded, metav1.ConditionFalse, operatorv1alpha1.ReasonReady, "") ||
				externalsecrets.Status.SetCondition(operatorv1alpha1.Ready, metav1.ConditionFalse, operatorv1alpha1.ReasonInProgress, fmt.Sprintf("reconciliation failed, retrying: %v", err)) {
				errUpdate = r.updateCondition(externalsecrets, err)
			}
			return ctrl.Result{RequeueAfter: defaultRequeueTime}, fmt.Errorf("failed to reconcile %q external-secrets deployment: %w", req, errUpdate)
		}
	}

	if externalsecrets.Status.SetCondition(operatorv1alpha1.Degraded, metav1.ConditionFalse, operatorv1alpha1.ReasonReady, "") ||
		externalsecrets.Status.SetCondition(operatorv1alpha1.Ready, metav1.ConditionTrue, operatorv1alpha1.ReasonReady, "reconciliation successful") {
		errUpdate = r.updateCondition(externalsecrets, nil)
	}
	return ctrl.Result{}, errUpdate
}

// cleanUp handles deletion of external-secrets.openshift.operator.io gracefully.
func (r *ExternalSecretsReconciler) cleanUp(externalsecrets *operatorv1alpha1.ExternalSecretsOperator) (bool, error) {
	r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeWarning, "RemoveDeployment", "%s/%s external-secrets marked for deletion, remove reference in  deployment and remove all resources created for deployment", externalsecrets.GetNamespace(), externalsecrets.GetName())
	return false, nil
}
