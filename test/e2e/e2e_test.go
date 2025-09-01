package e2e

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetconsts "sigs.k8s.io/jobset/pkg/constants"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/util"
)

const (
	torchRuntime     = "torch-distributed"
	deepSpeedRuntime = "deepspeed-distributed"
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
	ginkgo.When("Creating TrainJob to perform the PyTorch workload", func() {
		// Verify the `torch-distributed` ClusterTrainingRuntime.
		ginkgo.It("should create TrainJob with PyTorch runtime reference", func() {
			// Create a TrainJob.
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-test-torch").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), torchRuntime).
				Obj()

			ginkgo.By("Create a TrainJob with torch-distributed runtime reference", func() {
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			// Wait for jobs to become active
			ginkgo.By("Wait for TrainJob jobs to become active", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Status.JobsStatus).Should(gomega.BeComparableTo([]trainer.JobStatus{
						{
							Name:      constants.Node,
							Ready:     0,
							Succeeded: 0,
							Failed:    0,
							Active:    1,
							Suspended: 0,
						},
					}, util.SortJobsStatus))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			// Wait for TrainJob to be in Succeeded status with all jobs succeeded.
			ginkgo.By("Wait for TrainJob to be in Succeeded status with all jobs succeeded", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Status.Conditions).Should(gomega.BeComparableTo([]metav1.Condition{
						{
							Type:    trainer.TrainJobComplete,
							Status:  metav1.ConditionTrue,
							Reason:  jobsetconsts.AllJobsCompletedReason,
							Message: jobsetconsts.AllJobsCompletedMessage,
						},
					}, util.IgnoreConditions))
					g.Expect(gotTrainJob.Status.JobsStatus).Should(gomega.BeComparableTo([]trainer.JobStatus{
						{
							Name:      constants.Node,
							Ready:     0,
							Succeeded: 1,
							Failed:    0,
							Active:    0,
							Suspended: 0,
						},
					}, util.SortJobsStatus))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})
		})
	})

	ginkgo.When("Creating TrainJob to perform OpenMPI workload", func() {
		// Verify the `deepspeed-distributed` ClusterTrainingRuntime.
		ginkgo.It("should create TrainJob with DeepSpeed runtime reference", func() {
			// Create a TrainJob.
			trainJob := testingutil.MakeTrainJobWrapper(ns.Name, "e2e-test-deepspeed").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), deepSpeedRuntime).
				Obj()

			ginkgo.By("Create a TrainJob with deepspeed-distributed runtime reference", func() {
				gomega.Expect(k8sClient.Create(ctx, trainJob)).Should(gomega.Succeed())
			})

			// Wait for jobs to become active
			ginkgo.By("Wait for TrainJob jobs to become active", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Status.JobsStatus).Should(gomega.BeComparableTo([]trainer.JobStatus{
						{
							Name:      constants.Launcher,
							Ready:     0,
							Succeeded: 0,
							Failed:    0,
							Active:    1,
							Suspended: 0,
						},
						{
							Name:      constants.Node,
							Ready:     0,
							Succeeded: 0,
							Failed:    0,
							Active:    1,
							Suspended: 0,
						},
					}, util.SortJobsStatus))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})

			// Wait for TrainJob to be in Succeeded status.
			ginkgo.By("Wait for TrainJob to be in Succeeded status", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					gotTrainJob := &trainer.TrainJob{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(trainJob), gotTrainJob)).Should(gomega.Succeed())
					g.Expect(gotTrainJob.Status.Conditions).Should(gomega.BeComparableTo([]metav1.Condition{
						{
							Type:    trainer.TrainJobComplete,
							Status:  metav1.ConditionTrue,
							Reason:  jobsetconsts.AllJobsCompletedReason,
							Message: jobsetconsts.AllJobsCompletedMessage,
						},
					}, util.IgnoreConditions))
					g.Expect(gotTrainJob.Status.JobsStatus).Should(gomega.BeComparableTo([]trainer.JobStatus{
						{
							Name:      constants.Launcher,
							Ready:     0,
							Succeeded: 1,
							Failed:    0,
							Active:    0,
							Suspended: 0,
						},
						{
							Name:      constants.Node,
							Ready:     0,
							Succeeded: 0,
							Failed:    0,
							Active:    0,
							Suspended: 0,
						},
					}, util.SortJobsStatus))
				}, util.TimeoutE2E, util.Interval).Should(gomega.Succeed())
			})
		})
	})
})
