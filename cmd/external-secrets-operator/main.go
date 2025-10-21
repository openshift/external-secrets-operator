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

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/textlogger"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/operator"
	// +kubebuilder:scaffold:imports
)

const (
	// metricsCertFileName is the certificate filename, which should be present
	// at the passed `metrics-cert-dir` path.
	metricsCertFileName = "tls.crt"

	// metricsKeyFileName is the private key filename, which should be present
	// at the passed `metrics-cert-dir` path.
	metricsKeyFileName = "tls.key"

	openshiftCACertificateFile = "/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt"
)

var (
	ctx      = context.Background()
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(networkingv1.AddToScheme(scheme))
	utilruntime.Must(rbacv1.AddToScheme(scheme))
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))
	utilruntime.Must(crdv1.AddToScheme(scheme))

	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		enableLeaderElection bool
		probeAddr            string
		logLevel             int
		enableHTTP2          bool
		secureMetrics        bool
		metricsAddr          string
		metricsCerts         string
		metricsTLSOpts       []func(*tls.Config)
		webhookTLSOpts       []func(*tls.Config)
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8443", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP. Set to 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.IntVar(&logLevel, "v", 1, "operator log verbosity")
	flag.StringVar(&metricsCerts, "metrics-cert-dir", "",
		"Secret name containing the certificates for the metrics server which should be present in operator namespace. "+
			"If not provided self-signed certificates will be used")
	flag.Parse()

	logConfig := textlogger.NewConfig(textlogger.Verbosity(logLevel))
	ctrl.SetLogger(textlogger.NewLogger(logConfig))

	if !enableHTTP2 {
		// if the enable-http2 flag is false (the default), http/2 should be disabled
		// due to its vulnerabilities.
		disableHTTP2 := func(c *tls.Config) {
			setupLog.Info("disabling http/2 for both metrics and webhook servers")
			c.NextProtos = []string{"http/1.1"}
		}
		metricsTLSOpts = append(metricsTLSOpts, disableHTTP2)
		webhookTLSOpts = append(webhookTLSOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress: metricsAddr,

		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		FilterProvider: filters.WithAuthenticationAndAuthorization,
	}

	if secureMetrics {
		setupLog.Info("setting up secure metrics server")
		metricsServerOptions.SecureServing = secureMetrics
		if metricsCerts != "" {
			if _, err := os.Stat(filepath.Join(metricsCerts, metricsCertFileName)); err != nil {
				setupLog.Error(err, "metrics certificate file not found at configured path")
				os.Exit(1)
			}
			if _, err := os.Stat(filepath.Join(metricsCerts, metricsKeyFileName)); err != nil {
				setupLog.Error(err, "metrics private key file not found at configured path")
				os.Exit(1)
			}
			setupLog.Info("using certificate key pair found in the configured dir for metrics server")
			metricsServerOptions.CertDir = metricsCerts
			metricsServerOptions.CertName = metricsCertFileName
			metricsServerOptions.KeyName = metricsKeyFileName
		}
		metricsTLSOpts = append(metricsTLSOpts, func(c *tls.Config) {
			certPool, err := x509.SystemCertPool()
			if err != nil {
				setupLog.Info("unable to load system certificate pool", "error", err)
				certPool = x509.NewCertPool()
			}
			openshiftCACert, err := os.ReadFile(openshiftCACertificateFile)
			if err != nil {
				setupLog.Error(err, "failed to read OpenShift CA certificate")
				os.Exit(1)
			}
			setupLog.Info("using openshift service CA for metrics client verification")
			certPool.AppendCertsFromPEM(openshiftCACert)
			c.ClientCAs = certPool
		})
		metricsServerOptions.TLSOpts = metricsTLSOpts
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "de6a4747.externalsecretsoperator.operator.openshift.io",
		Logger:                 ctrl.Log.WithName("operator-manager"),
	})
	if err != nil {
		setupLog.Error(err, "failed to create controller manager")
		os.Exit(1)
	}

	if err := operator.StartControllers(ctx, mgr); err != nil {
		setupLog.Error(err, "failed to start controllers")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "failed to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting the controller manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "failed to start controller manager")
		os.Exit(1)
	}
}
