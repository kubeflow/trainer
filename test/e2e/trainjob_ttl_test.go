package e2e

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/util"
)

var _ = ginkgo.Describe("TrainJob Lifecycle e2e", func() {
	var ns *corev1.Namespace

	ginkgo.BeforeEach(func() {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "e2e-lifecycle-",
			},
		}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())

		gomega.Eventually(func(g gomega.Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ns), ns)).Should(gomega.Succeed())
		}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.Delete(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.When("testing TrainJob TTL", func() {
		ginkgo.It("should delete the TrainJob automatically after it finishes if TTL is set", func() {
			var torchCR trainer.ClusterTrainingRuntime
			gomega.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: torchRuntime}, &torchCR)).Should(gomega.Succeed())

			ttl := int32(5)
			trainingRuntime := testingutil.MakeTrainingRuntimeWrapper(ns.Name, "ttl-runtime").
				RuntimeSpec(testingutil.MakeTrainingRuntimeSpecWrapper(torchCR.Spec).Obj()).
				TTLSecondsAfterFinished(&ttl).
				Obj()
			gomega.Expect(k8sClient.Create(ctx, trainingRuntime)).Should(gomega.Succeed())

			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-ttl-job").
				RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), trainingRuntime.Name).
				Obj()
			gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())

			trainJobKey := client.ObjectKeyFromObject(trainJob)

			ginkgo.By("Waiting for TrainJob to be deleted by TTL controller after finishing")
			gomega.Eventually(func(g gomega.Gomega) {
				err := k8sClient.Get(ctx, trainJobKey, &trainer.TrainJob{})
				g.Expect(client.IgnoreNotFound(err)).Should(gomega.Succeed())
				g.Expect(err).Should(gomega.HaveOccurred())
			}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
		})
	})

	ginkgo.When("testing TrainJob ActiveDeadlineSeconds", func() {
		ginkgo.It("should fail the TrainJob with DeadlineExceeded when active timeout expires", func() {
			deadline := int64(10)
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-deadline-job").
				RuntimeRef(trainer.GroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), torchRuntime).
				ActiveDeadlineSeconds(&deadline).
				Obj()
			
			trainJob.Spec.Trainer = &trainer.Trainer{
				Image: ptr.To("busybox"),
				Command: []string{"/bin/sh", "-c", "sleep 600"},
			}
			gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())

			trainJobKey := client.ObjectKeyFromObject(trainJob)

			ginkgo.By("Waiting for TrainJob to fail due to deadline")
			gomega.Eventually(func(g gomega.Gomega) {
				gotTrainJob := &trainer.TrainJob{}
				g.Expect(k8sClient.Get(ctx, trainJobKey, gotTrainJob)).Should(gomega.Succeed())
				g.Expect(gotTrainJob.Status.Conditions).Should(gomega.ContainElement(gomega.HaveField("Reason", trainer.TrainJobDeadlineExceededReason)))
			}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
		})
	})
})
