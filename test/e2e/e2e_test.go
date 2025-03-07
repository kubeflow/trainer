package e2e

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetconsts "sigs.k8s.io/jobset/pkg/constants"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/constants"
	testingutil "github.com/kubeflow/trainer/pkg/util/testing"
	"github.com/kubeflow/trainer/test/util"
)

const (
	torchRuntime = "torch-distributed"
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
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ns), ns)).Should(gomega.Succeed())
		}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
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
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-test").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), torchRuntime).
				Obj()

			ginkgo.By("Create a TrainJob with torch-distributed runtime reference", func() {
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			// Wait for TrainJob to be in Succeeded status.
			ginkgo.By("Wait for TrainJob to be in Succeeded status", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Status.Conditions).Should(gomega.BeComparableTo([]metav1.Condition{
						{
							Type:    trainer.TrainJobCreated,
							Status:  metav1.ConditionTrue,
							Reason:  trainer.TrainJobJobsCreationSucceededReason,
							Message: constants.TrainJobJobsCreationSucceededMessage,
						},
						{
							Type:    trainer.TrainJobComplete,
							Status:  metav1.ConditionTrue,
							Reason:  jobsetconsts.AllJobsCompletedReason,
							Message: jobsetconsts.AllJobsCompletedMessage,
						},
					}, util.IgnoreConditions))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})
		})
	})
})
