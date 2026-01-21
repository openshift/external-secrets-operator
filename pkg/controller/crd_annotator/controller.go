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

package crd_annotator

import (
	"context"
	"fmt"

	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	operatorclient "github.com/openshift/external-secrets-operator/pkg/controller/client"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

const (
	ControllerName = "crd-annotator"

	// requestEnqueueLabelKey is the label key name used for filtering reconcile
	// events to include only the resources created by the controller.
	requestEnqueueLabelKey = "external-secrets.io/component"

	// requestEnqueueLabelValue is the label value used for filtering reconcile
	// events to include only the resources created by the controller.
	requestEnqueueLabelValue = "controller"

	// reconcileObjectIdentifier is for identifying the object for which reconcile event
	// is received, based on which a specific action will be taken.
	reconcileObjectIdentifier = "external-secrets-obj"
)

// Reconciler reconciles metadata on the managed CRDs.
type Reconciler struct {
	operatorclient.CtrlClient

	log logr.Logger
}

func NewClient(ctx context.Context, m manager.Manager) (operatorclient.CtrlClient, error) {
	c, err := BuildCustomClient(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("failed to build custom client: %w", err)
	}
	return &operatorclient.CtrlClientImpl{
		Client: c,
	}, nil
}

// New is for building the reconciler instance consumed by the Reconcile method.
func New(ctx context.Context, mgr ctrl.Manager) (*Reconciler, error) {
	r := &Reconciler{
		log: ctrl.Log.WithName(ControllerName),
	}
	c, err := NewClient(ctx, mgr)
	if err != nil {
		return nil, err
	}
	r.CtrlClient = c
	return r, nil
}

// BuildCustomClient creates a custom client with a custom cache of required objects.
// The corresponding informers receive events for objects matching label criteria.
func BuildCustomClient(ctx context.Context, mgr ctrl.Manager) (client.Client, error) {
	managedResourceLabelReq, _ := labels.NewRequirement(requestEnqueueLabelKey, selection.Equals, []string{requestEnqueueLabelValue})
	managedResourceLabelReqSelector := labels.NewSelector().Add(*managedResourceLabelReq)

	customCacheOpts := cache.Options{
		HTTPClient: mgr.GetHTTPClient(),
		Scheme:     mgr.GetScheme(),
		Mapper:     mgr.GetRESTMapper(),
		ByObject: map[client.Object]cache.ByObject{
			&crdv1.CustomResourceDefinition{}: {
				Label: managedResourceLabelReqSelector,
			},
			&operatorv1alpha1.ExternalSecretsConfig{}: {},
		},
		ReaderFailOnMissingInformer: true,
	}
	customCache, err := cache.New(mgr.GetConfig(), customCacheOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to build custom cache: %w", err)
	}
	if _, err = customCache.GetInformer(ctx, &crdv1.CustomResourceDefinition{}); err != nil {
		return nil, err
	}
	if _, err = customCache.GetInformer(ctx, &operatorv1alpha1.ExternalSecretsConfig{}); err != nil {
		return nil, err
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
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	mapFunc := func(ctx context.Context, obj client.Object) []reconcile.Request {
		r.log.V(4).Info("received reconcile event", "object", fmt.Sprintf("%T", obj), "name", obj.GetName(), "namespace", obj.GetNamespace())

		var objName string
		objLabels := obj.GetLabels()
		if objLabels != nil {
			if objLabels[requestEnqueueLabelKey] == requestEnqueueLabelValue {
				objName = obj.GetName()
			}
		}
		if _, ok := obj.(*operatorv1alpha1.ExternalSecretsConfig); ok {
			objName = reconcileObjectIdentifier
		}
		if objName != "" {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name: objName,
					},
				},
			}
		}

		r.log.V(4).Info("object not of interest, ignoring reconcile event", "object", fmt.Sprintf("%T", obj), "name", obj.GetName(), "namespace", obj.GetNamespace())
		return []reconcile.Request{}
	}

	// predicate function to ignore events for objects not managed by controller.
	managedResources := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetLabels() != nil && object.GetLabels()[requestEnqueueLabelKey] == requestEnqueueLabelValue
	})
	managedResourcePredicate := builder.WithPredicates(managedResources, predicate.AnnotationChangedPredicate{})

	return ctrl.NewControllerManagedBy(mgr).
		Named(ControllerName).
		WatchesMetadata(&crdv1.CustomResourceDefinition{}, handler.EnqueueRequestsFromMapFunc(mapFunc), managedResourcePredicate).
		Watches(&operatorv1alpha1.ExternalSecretsConfig{}, handler.EnqueueRequestsFromMapFunc(mapFunc), builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// Reconcile is the reconciliation loop to manage the current state of managed CRDS
// to match the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.V(1).Info("reconciling", "request", req)

	// Fetch the externalsecretsconfigs.operator.openshift.io CR
	esc := &operatorv1alpha1.ExternalSecretsConfig{}
	key := types.NamespacedName{
		Name: common.ExternalSecretsConfigObjectName,
	}
	if err := r.Get(ctx, key, esc); err != nil {
		if errors.IsNotFound(err) {
			// NotFound errors, would mean the object hasn't been created yet and
			// not required to reconcile yet.
			r.log.V(1).Info("externalsecretsconfigs.operator.openshift.io object not found, skipping reconciliation", "key", key)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to fetch externalsecretsconfigs.operator.openshift.io %q during reconciliation: %w", key, err)
	}

	if common.IsInjectCertManagerAnnotationEnabled(esc) {
		return r.processReconcileRequest(ctx, esc, req.NamespacedName)
	}

	return ctrl.Result{}, nil
}

// processReconcileRequest is the reconciliation handler to manage the resources.
func (r *Reconciler) processReconcileRequest(ctx context.Context, esc *operatorv1alpha1.ExternalSecretsConfig, req types.NamespacedName) (ctrl.Result, error) {
	var oErr error = nil
	if req.Name == reconcileObjectIdentifier {
		if err := r.updateAnnotationsInAllCRDs(ctx); err != nil {
			oErr = fmt.Errorf("failed while updating annotations in all CRDs: %w", err)
		}
	} else {
		crd := &crdv1.CustomResourceDefinition{}
		if err := r.Get(ctx, req, crd); err != nil {
			oErr = fmt.Errorf("failed to fetch customresourcedefinitions.apiextensions.k8s.io %q during reconciliation: %w", req, err)
		} else if err := r.updateAnnotations(ctx, crd); err != nil {
			oErr = fmt.Errorf("failed to update annotations in %q: %w", req, err)
		}
	}

	if err := r.updateCondition(ctx, esc, oErr); err != nil {
		return ctrl.Result{}, utilerrors.NewAggregate([]error{err, oErr})
	}

	return ctrl.Result{}, oErr
}

// updateAnnotations is for updating the annotations on the managed CRDs.
func (r *Reconciler) updateAnnotations(ctx context.Context, crd *crdv1.CustomResourceDefinition) error {
	annotations := crd.GetAnnotations()
	if val, ok := annotations[common.CertManagerInjectCAFromAnnotation]; !ok || val != common.CertManagerInjectCAFromAnnotationValue {
		patch := client.RawPatch(types.MergePatchType,
			fmt.Appendf(nil, "{\"metadata\":{\"annotations\":{\"%s\":\"%s\"}}}",
				common.CertManagerInjectCAFromAnnotation, common.CertManagerInjectCAFromAnnotationValue),
		)
		if err := r.Patch(ctx, crd, patch); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) updateAnnotationsInAllCRDs(ctx context.Context) error {
	managedCRDList := &crdv1.CustomResourceDefinitionList{}
	crdLabelFilter := map[string]string{
		requestEnqueueLabelKey: requestEnqueueLabelValue,
	}
	if err := r.List(ctx, managedCRDList, client.MatchingLabels(crdLabelFilter)); err != nil {
		return fmt.Errorf("failed to list managed CRD resources: %w", err)
	}
	if len(managedCRDList.Items) == 0 {
		r.log.Info("list query to fetch managed CRD resources returned empty")
		return nil
	}

	for _, crd := range managedCRDList.Items {
		if err := r.updateAnnotations(ctx, &crd); err != nil {
			return fmt.Errorf("failed to update annotations in %q: %w", crd.GetName(), err)
		}
	}

	return nil
}

func (r *Reconciler) updateCondition(ctx context.Context, esc *operatorv1alpha1.ExternalSecretsConfig, err error) error {
	cond := metav1.Condition{
		Type:               operatorv1alpha1.UpdateAnnotation,
		ObservedGeneration: esc.GetGeneration(),
	}

	if err != nil {
		cond.Status = metav1.ConditionFalse
		cond.Reason = operatorv1alpha1.ReasonFailed
		cond.Message = fmt.Sprintf("failed to add annotations: %v", err.Error())
	} else {
		cond.Status = metav1.ConditionTrue
		cond.Reason = operatorv1alpha1.ReasonCompleted
		cond.Message = "successfully updated annotations"
	}

	if apimeta.SetStatusCondition(&esc.Status.Conditions, cond) {
		return r.updateStatus(ctx, esc)
	}

	return nil
}

// updateStatus is for updating the status subresource of externalsecretsconfigs.operator.openshift.io.
func (r *Reconciler) updateStatus(ctx context.Context, changed *operatorv1alpha1.ExternalSecretsConfig) error {
	namespacedName := client.ObjectKeyFromObject(changed)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		r.log.V(4).Info("updating externalsecretsconfigs.operator.openshift.io status", "request", namespacedName)
		current := &operatorv1alpha1.ExternalSecretsConfig{}
		if err := r.Get(ctx, namespacedName, current); err != nil {
			return fmt.Errorf("failed to fetch externalsecretsconfigs.operator.openshift.io %q for status update: %w", namespacedName, err)
		}
		changed.Status.DeepCopyInto(&current.Status)

		if err := r.StatusUpdate(ctx, current); err != nil {
			return fmt.Errorf("failed to update externalsecretsconfigs.operator.openshift.io %q status: %w", namespacedName, err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
