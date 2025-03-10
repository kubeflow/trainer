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

package framework

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlpkg "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	controller "github.com/kubeflow/trainer/pkg/controller"
	runtimecore "github.com/kubeflow/trainer/pkg/runtime/core"
	kubeflowwebhooks "github.com/kubeflow/trainer/pkg/webhooks"
)

type Framework struct {
	testEnv *envtest.Environment
	cancel  context.CancelFunc
}

func (f *Framework) Init() *rest.Config {
	ctrl.SetLogger(zap.New(zap.WriteTo(ginkgo.GinkgoWriter), zap.Level(zapcore.Level(-5)), zap.UseDevMode(true)))
	ginkgo.By("bootstrapping test environment")
	f.testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "manifests", "base", "crds"),
			filepath.Join("..", "..", "..", "manifests", "external-crds", "scheduler-plugins", "crd.yaml"),
			filepath.Join("..", "..", "..", "manifests", "external-crds", "jobset-operator"),
		},
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "manifests", "base", "webhook", "validating_webhook_configuration.yaml")},
		},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := f.testEnv.Start()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(cfg).NotTo(gomega.BeNil())
	return cfg
}

func (f *Framework) RunManager(cfg *rest.Config) (context.Context, client.Client) {
	webhookInstallOpts := &f.testEnv.WebhookInstallOptions
	gomega.ExpectWithOffset(1, trainer.AddToScheme(scheme.Scheme)).NotTo(gomega.HaveOccurred())
	gomega.ExpectWithOffset(1, jobsetv1alpha2.AddToScheme(scheme.Scheme)).NotTo(gomega.HaveOccurred())
	gomega.ExpectWithOffset(1, schedulerpluginsv1alpha1.AddToScheme(scheme.Scheme)).NotTo(gomega.HaveOccurred())

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	gomega.ExpectWithOffset(1, k8sClient).NotTo(gomega.BeNil())

	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	mgr, err := ctrl.NewManager(cfg, manager.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0", // disable metrics to avoid conflicts between packages.
		},
		WebhookServer: webhook.NewServer(
			webhook.Options{
				Host:    webhookInstallOpts.LocalServingHost,
				Port:    webhookInstallOpts.LocalServingPort,
				CertDir: webhookInstallOpts.LocalServingCertDir,
			}),
	})
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "failed to create manager")

	runtimes, err := runtimecore.New(ctx, mgr.GetClient(), mgr.GetFieldIndexer())
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	gomega.ExpectWithOffset(1, runtimes).NotTo(gomega.BeNil())

	failedCtrlName, err := controller.SetupControllers(mgr, runtimes, ctrlpkg.Options{
		// controller-runtime v0.19+ validates controller names are unique, to make sure
		// exported Prometheus metrics for each controller do not conflict. The current check
		// relies on static state that's not compatible with testing execution model.
		// See the following resources for more context:
		// https://github.com/kubernetes-sigs/controller-runtime/pull/2902#issuecomment-2284194683
		// https://github.com/kubernetes-sigs/controller-runtime/issues/2994
		SkipNameValidation: ptr.To(true),
	})
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "controller", failedCtrlName)
	gomega.ExpectWithOffset(1, failedCtrlName).To(gomega.BeEmpty())

	failedWebhookName, err := kubeflowwebhooks.Setup(mgr, runtimes)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "webhook", failedWebhookName)
	gomega.ExpectWithOffset(1, failedWebhookName).To(gomega.BeEmpty())

	go func() {
		defer ginkgo.GinkgoRecover()
		err = mgr.Start(ctx)
		gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "failed to run manager")
	}()

	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOpts.LocalServingHost, webhookInstallOpts.LocalServingPort)
	gomega.Eventually(func(g gomega.Gomega) {
		var conn *tls.Conn
		conn, err = tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		g.Expect(err).Should(gomega.Succeed())
		g.Expect(conn.Close()).Should(gomega.Succeed())
	}).Should(gomega.Succeed())
	return ctx, k8sClient
}

func (f *Framework) Teardown() {
	ginkgo.By("tearing down the test environment")
	if f.cancel != nil {
		f.cancel()
	}
	gomega.ExpectWithOffset(1, f.testEnv.Stop()).NotTo(gomega.HaveOccurred())
}
