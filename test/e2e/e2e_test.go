package e2e

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	testingutil "github.com/kubeflow/trainer/pkg/util/testing"
)

var _ = ginkgo.Describe("TrainJob e2e", func() {
	// Each test runs in a separate namespace.
	var ns *corev1.Namespace

	// Create test namespace before each test.
	ginkgo.BeforeEach(func() {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "e2e-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())

		// Wait for namespace to exist before proceeding with test.
		gomega.Eventually(func(g gomega.Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns.Namespace, Name: ns.Name}, ns)).Shoud(gomega.Succeeded())
		}, timeout, interval).Should(gomega.Succeed())
	})

	// Delete test namespace after each test.
	ginkgo.AfterEach(func() {
		// Delete test namespace after each test.
		gomega.Expect(k8sClient.Delete(ctx, ns)).To(gomega.Succeed())
	})

	// These tests create TrainJob that reference supported runtime without any additional changes.
	ginkgo.When("creating TrainJob", func() {
		// Verify `torch-distributed` ClusterTrainingRuntime.
		ginkgo.It("should create TrainJob with PyTorch runtime reference", func() {
			// Create a TrainJob.
			trainJob := testTrainJob(ns.Name, "torch-distributed")
			trainJobKey := types.NamespacedName{Name: trainJob.Name, Namespace: trainJob.Namespace}

			ginkgo.By("Create a TrainJob with torch-distributed runtime reference", func() {
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			// Wait for TrainJob to be in Succeeded status.
			ginkgo.By("Wait for TrainJob to be in Succeeded status", func() {
				gomega.Eventually(func() bool {
					gomega.Expect(k8sClient.Get(ctx, trainJobKey, trainJob)).Should(gomega.Succeed())
					for _, c := range trainJob.Status.Conditions {
						if c.Type == trainer.TrainJobComplete && c.Status == metav1.ConditionTrue {
							return true
						}
					}
					return false
				}, timeout, interval).Should(gomega.Equal(true))
			})
		})
	})
})

func testTrainJob(namespace, runtimeRef string) *trainer.TrainJob {
	return testingutil.MakeTrainJobWrapper(namespace, "e2e-test").
		RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), runtimeRef).
		Obj()
}
