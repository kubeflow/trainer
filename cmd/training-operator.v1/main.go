/*
Copyright 2021.

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
	"errors"
	"flag"
	"net/http"
	"os"
	"strings"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/util/flowcontrol"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	"volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	volcanoclient "volcano.sh/apis/pkg/client/clientset/versioned"

	kubeflowv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	"github.com/kubeflow/training-operator/pkg/cert"
	"github.com/kubeflow/training-operator/pkg/config"
	controllerv1 "github.com/kubeflow/training-operator/pkg/controller.v1"
	"github.com/kubeflow/training-operator/pkg/controller.v1/common"
	"github.com/kubeflow/training-operator/pkg/webhooks"
	//+kubebuilder:scaffold:imports
)

const (
	// EnvKubeflowNamespace is an environment variable for namespace when deployed on kubernetes
	EnvKubeflowNamespace = "KUBEFLOW_NAMESPACE"

	webhookConfigurationName = "validator.training-operator.kubeflow.org"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubeflowv1.AddToScheme(scheme))
	utilruntime.Must(v1beta1.AddToScheme(scheme))
	utilruntime.Must(schedulerpluginsv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var leaderElectionID string
	var probeAddr string
	var enabledSchemes controllerv1.EnabledSchemes
	var gangSchedulerName string
	var namespace string
	var controllerThreads int
	var webhookServerPort int
	var webhookServiceName string
	var webhookSecretName string
	var clientQps int
	var clientBurst int

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&leaderElectionID, "leader-election-id", "1ca428e5.training-operator.kubeflow.org", "The ID for leader election.")
	flag.Var(&enabledSchemes, "enable-scheme", "Enable scheme(s) as --enable-scheme=tfjob --enable-scheme=pytorchjob, case insensitive."+
		" Now supporting TFJob, PyTorchJob, XGBoostJob, PaddleJob, JAXJob. By default, all supported schemes will be enabled.")
	flag.StringVar(&gangSchedulerName, "gang-scheduler-name", "", "Now Supporting volcano and scheduler-plugins."+
		" Note: If you set another scheduler name, the training-operator assumes it's the scheduler-plugins.")
	flag.StringVar(&namespace, "namespace", os.Getenv(EnvKubeflowNamespace), "The namespace to monitor kubeflow jobs. If unset, it monitors all namespaces cluster-wide."+
		"If set, it only monitors kubeflow jobs in the given namespace.")
	flag.IntVar(&controllerThreads, "controller-threads", 1, "Number of worker threads used by the controller.")
	flag.IntVar(&clientQps, "kube-api-qps", 20, "QPS indicates the maximum QPS to the master from this client.")
	flag.IntVar(&clientBurst, "kube-api-burst", 30, "Maximum burst for throttle.")
	// PyTorch related flags
	flag.StringVar(&config.Config.PyTorchInitContainerImage, "pytorch-init-container-image",
		config.PyTorchInitContainerImageDefault, "The image for pytorch init container")
	flag.StringVar(&config.Config.PyTorchInitContainerTemplateFile, "pytorch-init-container-template-file",
		config.PyTorchInitContainerTemplateFileDefault, "The template file for pytorch init container")
	flag.IntVar(&config.Config.PyTorchInitContainerMaxTries, "pytorch-init-container-max-tries",
		config.PyTorchInitContainerMaxTriesDefault, "The number of tries for the pytorch init container")

	// MPI related flags
	flag.StringVar(&config.Config.MPIKubectlDeliveryImage, "mpi-kubectl-delivery-image",
		config.MPIKubectlDeliveryImageDefault, "The image for mpi launcher init container")
	flag.BoolVar(&config.Config.DisableMPIRBACManagement, "disable-mpi-rbac-management", false,
		"When set to true, disables the MPIJob controller's management of launcher RBAC resources (ServiceAccount, Role, RoleBinding) and ServiceAccountName injection. Users must provide their own ServiceAccount.")

	// Cert generation flags
	flag.IntVar(&webhookServerPort, "webhook-server-port", 9443, "Endpoint port for the webhook server.")
	flag.StringVar(&webhookServiceName, "webhook-service-name", "training-operator", "Name of the Service used as part of the DNSName")
	flag.StringVar(&webhookSecretName, "webhook-secret-name", "training-operator-webhook-cert", "Name of the Secret to store CA  and server certs")

	opts := zap.Options{
		Development:     true,
		StacktraceLevel: zapcore.DPanicLevel,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	var cacheOpts cache.Options
	if namespace != "" {
		cacheOpts = cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				namespace: {},
			},
		}
	}

	cfg := ctrl.GetConfigOrDie()
	cfg.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(float32(clientQps), clientBurst)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: webhookServerPort,
		}),
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       leaderElectionID,
		Cache:                  cacheOpts,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	certsReady := make(chan struct{})
	defer close(certsReady)
	certGenerationConfig := cert.Config{
		WebhookSecretName:        webhookSecretName,
		WebhookServiceName:       webhookServiceName,
		WebhookConfigurationName: webhookConfigurationName,
	}
	if err = cert.ManageCerts(mgr, certGenerationConfig, certsReady); err != nil {
		setupLog.Error(err, "Unable to set up cert rotation")
		os.Exit(1)
	}

	setupProbeEndpoints(mgr, certsReady)
	// Set up controllers using goroutines to start the manager quickly.
	go setupControllers(mgr, enabledSchemes, gangSchedulerName, controllerThreads, certsReady)

	//+kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupControllers(mgr ctrl.Manager, enabledSchemes controllerv1.EnabledSchemes, gangSchedulerName string, controllerThreads int, certsReady <-chan struct{}) {
	setupLog.Info("Waiting for certificate generation to complete")
	<-certsReady
	setupLog.Info("Certs ready")

	setupLog.Info("registering controllers...")
	// Prepare GangSchedulingSetupFunc
	gangSchedulingSetupFunc := common.GenNonGangSchedulerSetupFunc()
	if strings.EqualFold(gangSchedulerName, string(common.GangSchedulerVolcano)) {
		cfg := mgr.GetConfig()
		volcanoClientSet := volcanoclient.NewForConfigOrDie(cfg)
		gangSchedulingSetupFunc = common.GenVolcanoSetupFunc(volcanoClientSet)
		gvk := v1beta1.SchemeGroupVersion.WithKind("PodGroup")
		validateCRD(mgr, gvk)
	} else if gangSchedulerName != "" {
		gangSchedulingSetupFunc = common.GenSchedulerPluginsSetupFunc(mgr.GetClient(), gangSchedulerName)
		gvk := schedulerpluginsv1alpha1.SchemeGroupVersion.WithKind("PodGroup")
		validateCRD(mgr, gvk)
	}

	// TODO: We need a general manager. all rest reconciler addsToManager
	// Based on the user configuration, we start different controllers
	if enabledSchemes.Empty() {
		enabledSchemes.FillAll()
	}
	errMsg := "failed to set up controllers"
	for _, s := range enabledSchemes {
		setupReconcilerFunc, supportedReconciler := controllerv1.SupportedSchemeReconciler[s]
		if !supportedReconciler {
			setupLog.Error(errors.New(errMsg), "scheme is not supported", "scheme", s)
			os.Exit(1)
		}
		if err := setupReconcilerFunc(mgr, gangSchedulingSetupFunc, controllerThreads); err != nil {
			setupLog.Error(errors.New(errMsg), "unable to create controller", "scheme", s)
			os.Exit(1)
		}
		setupWebhookFunc, supportedWebhook := webhooks.SupportedSchemeWebhook[s]
		if !supportedWebhook {
			setupLog.Error(errors.New(errMsg), "scheme is not supported", "scheme", s)
			os.Exit(1)
		}
		if err := setupWebhookFunc(mgr); err != nil {
			setupLog.Error(errors.New(errMsg), "unable to start webhook server", "scheme", s)
			os.Exit(1)
		}
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
	// that the early requests are not rejected during the traininig-operator's startup.
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

func validateCRD(mgr ctrl.Manager, gvk schema.GroupVersionKind) {
	_, err := mgr.GetRESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		if meta.IsNoMatchError(err) {
			setupLog.Error(err, "crd might be missing, please install crd", "apiVersion", gvk.GroupVersion().String(), "kind", gvk.Kind)
			os.Exit(1)
		}
		setupLog.Error(err, "unable to get crd", "apiVersion", gvk.GroupVersion().String(), "kind", gvk.Kind)
		os.Exit(1)
	}
}
