/*
Copyright 2024 The Kubeflow Authors.

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
	"crypto/tls"
	"errors"
	"flag"
	"net/http"
	"os"

	zaplog "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlpkg "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/controller"
	"github.com/kubeflow/trainer/pkg/runtime"
	runtimecore "github.com/kubeflow/trainer/pkg/runtime/core"
	"github.com/kubeflow/trainer/pkg/util/cert"
	webhooks "github.com/kubeflow/trainer/pkg/webhooks"
)

const (
	webhookConfigurationName = "validator.trainer.kubeflow.org"
)

var (
	scheme   = apiruntime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(trainer.AddToScheme(scheme))
	utilruntime.Must(jobsetv1alpha2.AddToScheme(scheme))
	utilruntime.Must(schedulerpluginsv1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var webhookServerPort int
	var webhookServiceName string
	var webhookSecretName string
	var tlsOpts []func(*tls.Config)

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	// Cert generation flags
	flag.IntVar(&webhookServerPort, "webhook-server-port", 9443, "Endpoint port for the webhook server.")
	flag.StringVar(&webhookServiceName, "webhook-service-name", "kubeflow-trainer-controller-manager", "Name of the Service used as part of the DNSName")
	flag.StringVar(&webhookSecretName, "webhook-secret-name", "kubeflow-trainer-webhook-cert", "Name of the Secret to store CA  and server certs")

	opts := zap.Options{
		TimeEncoder: zapcore.RFC3339NanoTimeEncoder,
		ZapOpts:     []zaplog.Option{zaplog.AddCaller()},
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if !enableHTTP2 {
		// if the enable-http2 flag is false (the default), http/2 should be disabled
		// due to its vulnerabilities. More specifically, disabling http/2 will
		// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
		// Rapid Reset CVEs. For more information see:
		// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
		// - https://github.com/advisories/GHSA-4374-p667-p6c8
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			setupLog.Info("disabling http/2")
			c.NextProtos = []string{"http/1.1"}
		})
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Client: client.Options{
			Cache: &client.CacheOptions{
				Unstructured: true,
			},
		},
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    webhookServerPort,
			TLSOpts: tlsOpts,
		}),
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	certsReady := make(chan struct{})
	if err = cert.ManageCerts(mgr, cert.Config{
		WebhookSecretName:        webhookSecretName,
		WebhookServiceName:       webhookServiceName,
		WebhookConfigurationName: webhookConfigurationName,
	}, certsReady); err != nil {
		setupLog.Error(err, "unable to set up cert rotation")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	setupProbeEndpoints(mgr, certsReady)
	runtimes, err := runtimecore.New(ctx, mgr.GetClient(), mgr.GetFieldIndexer())
	if err != nil {
		setupLog.Error(err, "Could not initialize runtimes")
		os.Exit(1)
	}
	// Set up controllers using goroutines to start the manager quickly.
	go setupControllers(mgr, runtimes, certsReady)

	setupLog.Info("Starting manager")
	if err = mgr.Start(ctx); err != nil {
		setupLog.Error(err, "Could not run manager")
		os.Exit(1)
	}
}

func setupControllers(mgr ctrl.Manager, runtimes map[string]runtime.Runtime, certsReady <-chan struct{}) {
	setupLog.Info("Waiting for certificate generation to complete")
	<-certsReady
	setupLog.Info("Certs ready")

	if failedCtrlName, err := controller.SetupControllers(mgr, runtimes, ctrlpkg.Options{}); err != nil {
		setupLog.Error(err, "Could not create controller", "controller", failedCtrlName)
		os.Exit(1)
	}
	if failedWebhook, err := webhooks.Setup(mgr, runtimes); err != nil {
		setupLog.Error(err, "Could not create webhook", "webhook", failedWebhook)
		os.Exit(1)
	}
}

func setupProbeEndpoints(mgr ctrl.Manager, certsReady <-chan struct{}) {
	defer setupLog.Info("Probe endpoints are configured on healthz and readyz")

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	// Wait for the webhook server to be listening before advertising the
	// training-operator replica as ready. This allows users to wait with sending the first
	// requests, requiring webhooks, until the training-operator deployment is available, so
	// that the early requests are not rejected during the training-operator's startup.
	// We wrap the call to GetWebhookServer in a closure to delay calling
	// the function, otherwise a not fully-initialized webhook server (without
	// ready certs) fails the start of the manager.
	if err := mgr.AddReadyzCheck("readyz", func(req *http.Request) error {
		select {
		case <-certsReady:
			return mgr.GetWebhookServer().StartedChecker()(req)
		default:
			return errors.New("certificates are not ready")
		}
	}); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
}
