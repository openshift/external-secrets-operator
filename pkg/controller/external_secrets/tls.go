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
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	configv1 "github.com/openshift/api/config/v1"

	"github.com/openshift/external-secrets-operator/pkg/tlsprofile"
)

// apiServerTLSProfileChangedPredicate admits apiserver.config.openshift.io
// events only when spec.tlsSecurityProfile changes, so unrelated apiserver
// updates do not trigger operand reconciliation. Create/Delete/Generic events
// fall through to the default (admit) behaviour of predicate.Funcs.
var apiServerTLSProfileChangedPredicate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldAPIServer, ok := e.ObjectOld.(*configv1.APIServer)
		if !ok {
			return false
		}
		newAPIServer, ok := e.ObjectNew.(*configv1.APIServer)
		if !ok {
			return false
		}
		return !reflect.DeepEqual(oldAPIServer.Spec.TLSSecurityProfile, newAPIServer.Spec.TLSSecurityProfile)
	},
}

// resolveOperandTLSProfile resolves the honored cluster TLS profile for the
// external-secrets operand deployments, logging the effective settings. A nil
// spec (with nil error) means no cluster profile should be enforced and existing
// operand TLS settings must be left unchanged.
//
// TODO: once the upstream external-secrets operand supports --tls-min-version,
// --tls-ciphers, and --tls-curve-preferences flags, use the returned spec to
// inject the flags into the operand container args.
func (r *Reconciler) resolveOperandTLSProfile() (*configv1.TLSProfileSpec, error) {
	tlsSpec, err := tlsprofile.ResolveHonoredTLSProfile(
		r.ctx,
		tlsprofile.NewClientReaderAPIServerFetch(r.CtrlClient),
		"external-secrets",
		tlsprofile.FetchErrorPropagateExceptNotFound,
	)
	if err != nil {
		r.log.Error(err, "failed to resolve cluster TLS profile")
		return nil, err
	}
	if tlsSpec != nil {
		r.log.V(2).Info("resolved cluster TLS profile for operand deployments",
			"minTLSVersion", tlsSpec.MinTLSVersion,
			"cipherCount", len(tlsSpec.Ciphers))

	}
	return tlsSpec, nil
}
