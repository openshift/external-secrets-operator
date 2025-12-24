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

package external_secrets_manager

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	operatorclient "github.com/openshift/external-secrets-operator/pkg/controller/client"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

const (
	ControllerName = "external-secrets-manager"

	// finalizer name for externalsecretsmanagers.operator.openshift.io resource.
	finalizer = "externalsecretsmanagers.operator.openshift.io/" + ControllerName
)

var (
	externalSecretsControllerId = fmt.Sprintf("externalsecretsconfigs.%s/%s", operatorv1alpha1.GroupVersion.Group, operatorv1alpha1.GroupVersion.Version)
)

// Reconciler reconciles externalsecretsmanagers.operator.openshift.io CR.
type Reconciler struct {
	operatorclient.CtrlClient

	Scheme        *runtime.Scheme
	ctx           context.Context
	eventRecorder record.EventRecorder
	log           logr.Logger
	now           *common.Now
	esc           *operatorv1alpha1.ExternalSecretsConfig
}

// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecretsmanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecretsmanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=externalsecretsmanagers/finalizers,verbs=update

// New is for building the reconciler instance consumed by the Reconcile method.
func New(ctx context.Context, mgr ctrl.Manager) *Reconciler {
	return &Reconciler{
		CtrlClient:    NewClient(mgr),
		Scheme:        mgr.GetScheme(),
		ctx:           ctx,
		eventRecorder: mgr.GetEventRecorderFor(ControllerName),
		log:           ctrl.Log.WithName(ControllerName),
		now:           &common.Now{},
	}
}

func NewClient(m manager.Manager) operatorclient.CtrlClient {
	return &operatorclient.CtrlClientImpl{
		Client: m.GetClient(),
	}
}

// SetupWithManager is for creating a controller instance with predicates and event filters.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	statusUpdatePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj := e.ObjectOld.(*operatorv1alpha1.ExternalSecretsConfig)
			newObj := e.ObjectNew.(*operatorv1alpha1.ExternalSecretsConfig)
			return !reflect.DeepEqual(oldObj.Status, newObj.Status)
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ExternalSecretsManager{}).
		Named(ControllerName).
		Watches(&operatorv1alpha1.ExternalSecretsConfig{}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(statusUpdatePredicate)).
		Complete(r)
}

// Reconcile is the reconciliation loop to manage the current state of externalsecretsmanager CR.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.V(1).Info("reconciling", "request", req)

	// Fetch the externalsecretsmanagers.operator.openshift.io CR
	esm := &operatorv1alpha1.ExternalSecretsManager{}
	key := types.NamespacedName{
		Name: common.ExternalSecretsManagerObjectName,
	}
	if err := r.Get(ctx, key, esm); err != nil {
		r.now.Do(func() {
			r.eventRecorder.Eventf(esm, corev1.EventTypeWarning, "Read", "failed to fetch externalsecretsmanagers.operator.openshift.io %q", key)
		})
		return ctrl.Result{RequeueAfter: common.DefaultRequeueTime}, fmt.Errorf("failed to fetch externalsecretsmanagers.operator.openshift.io %q during reconciliation: %w", key, err)
	}
	r.now.Reset()

	if !esm.DeletionTimestamp.IsZero() {
		r.log.V(1).Info("externalsecretsmanagers.operator.openshift.io is marked for deletion", "key", key)

		if err := common.RemoveFinalizer(ctx, esm, r.CtrlClient, finalizer); err != nil {
			return ctrl.Result{}, err
		}

		r.log.V(1).Info("removed finalizer, cleanup complete", "key", key)
		return ctrl.Result{}, nil
	}

	// Set finalizers on the externalsecretsmanagers.operator.openshift.io resource
	if err := common.AddFinalizer(ctx, esm, r.CtrlClient, finalizer); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update %q externalsecretsmanagers.operator.openshift.io with finalizers: %w", key, err)
	}

	// Fetch the externalsecretsconfigs.operator.openshift.io CR
	r.esc = new(operatorv1alpha1.ExternalSecretsConfig)
	key = types.NamespacedName{
		Name: common.ExternalSecretsConfigObjectName,
	}
	if err := r.Get(ctx, key, r.esc); err != nil {
		if errors.IsNotFound(err) {
			// NotFound errors, would mean the object hasn't been created yet and
			// not required to reconcile yet.
			r.log.V(1).Info("externalsecretsconfigs.operator.openshift.io object not found, skipping reconciliation", "key", key)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to fetch externalsecretsconfigs.operator.openshift.io %q during reconciliation: %w", key, err)
	}

	return r.processReconcileRequest(esm)
}

// processReconcileRequest is the reconciliation handler to manage the resources.
func (r *Reconciler) processReconcileRequest(esm *operatorv1alpha1.ExternalSecretsManager) (ctrl.Result, error) {
	statusUpdated := false
	if esm.Status.ControllerStatuses == nil {
		esm.Status.ControllerStatuses = make([]operatorv1alpha1.ControllerStatus, 0)
	}
	if r.esc != nil && len(r.esc.Status.Conditions) > 0 {
		for _, esCond := range r.esc.Status.Conditions {
			if r.updateStatusCondition(esm, externalSecretsControllerId, esCond) {
				statusUpdated = true
			}
		}
	}

	if statusUpdated {
		if err := r.updateStatus(r.ctx, esm); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update externalsecretsmanagers.operator.openshift.io status: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// getControllerCondition returns the status condition of a specific controller.
func getControllerCondition(controllerName string, esm *operatorv1alpha1.ExternalSecretsManager) *operatorv1alpha1.ControllerStatus {
	for i, s := range esm.Status.ControllerStatuses {
		if s.Name == controllerName {
			return &esm.Status.ControllerStatuses[i]
		}
	}
	status := &operatorv1alpha1.ControllerStatus{
		Name:       controllerName,
		Conditions: make([]operatorv1alpha1.Condition, 0),
	}
	esm.Status.ControllerStatuses = append(esm.Status.ControllerStatuses, *status)
	return status
}

// updateStatusCondition updates the status condition of a specific controller.
func (r *Reconciler) updateStatusCondition(esm *operatorv1alpha1.ExternalSecretsManager, controllerName string, updCondition metav1.Condition) bool {
	status := getControllerCondition(controllerName, esm)

	found, condUpdated := false, false
	for i, c := range status.Conditions {
		if c.Type == updCondition.Type {
			found = true
			if c.Status != updCondition.Status || c.Message != updCondition.Message {
				status.Conditions[i].Status = updCondition.Status
				status.Conditions[i].Message = updCondition.Message
				condUpdated = true
			}
		}
	}

	if !found {
		status.Conditions = append(status.Conditions, operatorv1alpha1.Condition{
			Type:    updCondition.Type,
			Status:  updCondition.Status,
			Message: updCondition.Message,
		})
		condUpdated = true
	}

	status.ObservedGeneration = updCondition.ObservedGeneration
	esm.Status.LastTransitionTime = metav1.Now()

	return condUpdated
}

// updateStatus is for updating the status subresource of externalsecretsmanagers.operator.openshift.io.
func (r *Reconciler) updateStatus(ctx context.Context, changed *operatorv1alpha1.ExternalSecretsManager) error {
	namespacedName := client.ObjectKeyFromObject(changed)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		r.log.V(4).Info("updating externalsecretsmanagers.operator.openshift.io status", "request", namespacedName)
		current := &operatorv1alpha1.ExternalSecretsManager{}
		if err := r.Get(ctx, namespacedName, current); err != nil {
			return fmt.Errorf("failed to fetch externalsecretsmanagers.operator.openshift.io %q for status update: %w", namespacedName, err)
		}
		changed.Status.DeepCopyInto(&current.Status)

		if err := r.StatusUpdate(ctx, current); err != nil {
			return fmt.Errorf("failed to update externalsecretsmanagers.operator.openshift.io %q status: %w", namespacedName, err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
