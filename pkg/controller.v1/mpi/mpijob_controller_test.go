// Copyright 2021 The Kubeflow Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mpi

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	common "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	kubeflowv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	ctlrconfig "github.com/kubeflow/training-operator/pkg/config"
	ctrlcommon "github.com/kubeflow/training-operator/pkg/controller.v1/common"
	commonutil "github.com/kubeflow/training-operator/pkg/util"
	"github.com/kubeflow/training-operator/pkg/util/testutil"
)

const (
	gpuResourceName         = "nvidia.com/gpu"
	extendedGPUResourceName = "vendor-domain/gpu"
)

func newMPIJobCommon(name string, startTime, completionTime *metav1.Time) *kubeflowv1.MPIJob {
	mpiJob := &kubeflowv1.MPIJob{
		TypeMeta: metav1.TypeMeta{APIVersion: kubeflowv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: kubeflowv1.MPIJobSpec{
			RunPolicy: common.RunPolicy{
				CleanPodPolicy: kubeflowv1.CleanPodPolicyPointer(kubeflowv1.CleanPodPolicyAll),
			},
			MPIReplicaSpecs: map[common.ReplicaType]*common.ReplicaSpec{
				kubeflowv1.MPIJobReplicaTypeWorker: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "foo",
									Image: "bar",
								},
							},
						},
					},
				},
				kubeflowv1.MPIJobReplicaTypeLauncher: {
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "foo",
									Image: "bar",
								},
							},
						},
					},
				},
			},
		},
		Status: common.JobStatus{},
	}

	if startTime != nil {
		mpiJob.Status.StartTime = startTime
	}
	if completionTime != nil {
		mpiJob.Status.CompletionTime = completionTime
	}

	return mpiJob
}

func newMPIJobOld(name string, replicas *int32, pusPerReplica int64, resourceName string, startTime, completionTime *metav1.Time) *kubeflowv1.MPIJob {
	mpiJob := newMPIJobCommon(name, startTime, completionTime)

	mpiJob.Spec.MPIReplicaSpecs[kubeflowv1.MPIJobReplicaTypeWorker].Replicas = replicas

	workerContainers := mpiJob.Spec.MPIReplicaSpecs[kubeflowv1.MPIJobReplicaTypeWorker].Template.Spec.Containers
	for i := range workerContainers {
		container := &workerContainers[i]
		container.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceName(resourceName): *resource.NewQuantity(pusPerReplica, resource.DecimalExponent),
			},
		}
	}

	return mpiJob
}

var newMPIJob = newMPIJobWithLauncher

func newMPIJobWithLauncher(name string, replicas *int32, pusPerReplica int64, resourceName string, startTime, completionTime *metav1.Time) *kubeflowv1.MPIJob {
	mpiJob := newMPIJobOld(name, replicas, pusPerReplica, resourceName, startTime, completionTime)

	mpiJob.Spec.MPIReplicaSpecs[kubeflowv1.MPIJobReplicaTypeLauncher].Replicas = ptr.To[int32](1)

	launcherContainers := mpiJob.Spec.MPIReplicaSpecs[kubeflowv1.MPIJobReplicaTypeLauncher].Template.Spec.Containers
	for i := range launcherContainers {
		container := &launcherContainers[i]
		container.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceName(resourceName): *resource.NewQuantity(pusPerReplica, resource.DecimalExponent),
			},
		}
	}

	return mpiJob
}

var _ = Describe("MPIJob controller", func() {
	Context("Test launcher is GPU launcher", func() {
		It("Should pass GPU Launcher verification", func() {
			By("By creating MPIJobs with various resource configuration")

			testCases := map[string]struct {
				gpu      string
				expected bool
			}{
				"isNvidiaGPU": {
					gpu:      gpuResourceName,
					expected: true,
				},
				"isExtendedGPU": {
					gpu:      extendedGPUResourceName,
					expected: true,
				},
				"notGPU": {
					gpu:      "vendor-domain/resourcetype",
					expected: false,
				},
			}

			startTime := metav1.Now()
			completionTime := metav1.Now()

			for testName, testCase := range testCases {
				mpiJob := newMPIJobWithLauncher("test-"+strings.ToLower(testName),
					ptr.To[int32](64), 1, testCase.gpu, &startTime, &completionTime)
				Expect(isGPULauncher(mpiJob) == testCase.expected).To(BeTrue())
			}
		})
	})

	Context("Test MPIJob with succeeded launcher Pod", func() {
		It("Should contains desired launcher ReplicaStatus", func() {
			By("By marking a launcher pod with Phase Succeeded")
			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			jobName := "test-launcher-succeeded"

			mpiJob := newMPIJobWithLauncher(jobName, ptr.To[int32](64), 1, gpuResourceName, &startTime, &completionTime)
			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			launcher := reconciler.newLauncher(mpiJob, "kubectl-delivery", isGPULauncher(mpiJob))
			launcher.Status.Phase = corev1.PodSucceeded

			launcherKey := types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      launcher.GetName(),
			}
			Eventually(func() error {
				launcherCreated := &corev1.Pod{}
				if err := testK8sClient.Get(ctx, launcherKey, launcherCreated); err != nil {
					return err
				}
				launcherCreated.Status.Phase = corev1.PodSucceeded
				return testK8sClient.Status().Update(ctx, launcherCreated)
			}, testutil.Timeout, testutil.Interval).Should(BeNil())

			created := &kubeflowv1.MPIJob{}
			launcherStatus := &common.ReplicaStatus{
				Active:    0,
				Succeeded: 1,
				Failed:    0,
			}
			Eventually(func() bool {
				err := testK8sClient.Get(ctx, types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: jobName}, created)
				if err != nil {
					return false
				}
				return ReplicaStatusMatch(created.Status.ReplicaStatuses, kubeflowv1.MPIJobReplicaTypeLauncher, launcherStatus)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
		})
	})

	Context("Test MPIJob with failed launcher Pod", func() {
		It("Should contains desired launcher ReplicaStatus", func() {
			By("By marking a launcher pod with Phase Failed")
			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			jobName := "test-launcher-failed"

			mpiJob := newMPIJobWithLauncher(jobName, ptr.To[int32](64), 1, gpuResourceName, &startTime, &completionTime)
			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			launcher := reconciler.newLauncher(mpiJob, "kubectl-delivery", isGPULauncher(mpiJob))
			launcherKey := types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      launcher.GetName(),
			}
			Eventually(func() error {
				launcherCreated := &corev1.Pod{}
				if err := testK8sClient.Get(ctx, launcherKey, launcherCreated); err != nil {
					return err
				}
				launcherCreated.Status.Phase = corev1.PodFailed
				return testK8sClient.Status().Update(ctx, launcherCreated)
			}, testutil.Timeout, testutil.Interval).Should(BeNil())

			launcherStatus := &common.ReplicaStatus{
				Active:    0,
				Succeeded: 0,
				Failed:    1,
			}
			created := &kubeflowv1.MPIJob{}
			Eventually(func() bool {
				err := testK8sClient.Get(ctx, types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: jobName}, created)
				if err != nil {
					return false
				}
				return ReplicaStatusMatch(created.Status.ReplicaStatuses, kubeflowv1.MPIJobReplicaTypeLauncher, launcherStatus)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
		})
	})

	Context("Test MPIJob with succeeded launcher pod", func() {
		It("Should contain desired ReplicaStatuses for worker", func() {
			By("By marking the launcher Pod as Succeeded")
			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			jobName := "test-launcher-succeeded2"

			mpiJob := newMPIJobWithLauncher(jobName, ptr.To[int32](64), 1, gpuResourceName, &startTime, &completionTime)
			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			launcher := reconciler.newLauncher(mpiJob, "kubectl-delivery", isGPULauncher(mpiJob))
			launcher.Status.Phase = corev1.PodSucceeded

			launcherKey := types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      launcher.GetName(),
			}
			Eventually(func() error {
				launcherCreated := &corev1.Pod{}
				if err := testK8sClient.Get(ctx, launcherKey, launcherCreated); err != nil {
					return err
				}
				launcherCreated.Status.Phase = corev1.PodSucceeded
				return testK8sClient.Status().Update(ctx, launcherCreated)
			}, testutil.Timeout, testutil.Interval).Should(BeNil())

			created := &kubeflowv1.MPIJob{}
			launcherStatus := &common.ReplicaStatus{
				Active:    0,
				Succeeded: 0,
				Failed:    0,
			}
			Eventually(func() bool {
				err := testK8sClient.Get(ctx, types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: jobName}, created)
				if err != nil {
					return false
				}
				return ReplicaStatusMatch(created.Status.ReplicaStatuses, kubeflowv1.MPIJobReplicaTypeWorker, launcherStatus)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
		})
	})

	Context("Test MPIJob with Running launcher Pod and Pending worker Pods", func() {
		It("Should contain desired ReplicaStatuses", func() {
			By("By marking an active launcher pod and pending worker pods")

			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			jobName := "test-launcher-running-worker-pending"

			var replicas int32 = 8
			mpiJob := newMPIJobWithLauncher(jobName, &replicas, 1, gpuResourceName, &startTime, &completionTime)
			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			launcher := reconciler.newLauncher(mpiJob, "kubectl-delivery", isGPULauncher(mpiJob))
			launcherKey := types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      launcher.GetName(),
			}
			Eventually(func() error {
				launcherCreated := &corev1.Pod{}
				if err := testK8sClient.Get(ctx, launcherKey, launcherCreated); err != nil {
					return err
				}
				launcherCreated.Status.Phase = corev1.PodRunning
				return testK8sClient.Status().Update(ctx, launcherCreated)
			}, testutil.Timeout, testutil.Interval).Should(BeNil())

			for i := 0; i < int(replicas); i++ {
				name := fmt.Sprintf("%s-%d", mpiJob.Name+workerSuffix, i)
				worker := reconciler.newWorker(mpiJob, name)
				workerKey := types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      worker.GetName(),
				}
				Eventually(func() error {
					workerCreated := &corev1.Pod{}
					if err := testK8sClient.Get(ctx, workerKey, workerCreated); err != nil {
						return err
					}
					workerCreated.Status.Phase = corev1.PodPending
					return testK8sClient.Status().Update(ctx, workerCreated)
				}, testutil.Timeout, testutil.Interval).Should(BeNil())
			}

			key := types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      jobName,
			}
			launcherStatus := &common.ReplicaStatus{
				Active:    1,
				Succeeded: 0,
				Failed:    0,
			}
			workerStatus := &common.ReplicaStatus{
				Active:    0,
				Succeeded: 0,
				Failed:    0,
			}
			Eventually(func() bool {
				created := &kubeflowv1.MPIJob{}
				err := testK8sClient.Get(ctx, key, created)
				if err != nil {
					return false
				}
				return ReplicaStatusMatch(created.Status.ReplicaStatuses, kubeflowv1.MPIJobReplicaTypeLauncher,
					launcherStatus) && ReplicaStatusMatch(created.Status.ReplicaStatuses, kubeflowv1.MPIJobReplicaTypeWorker,
					workerStatus)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
		})
	})

	Context("Test MPIJob with Running launcher Pod and Running worker Pods", func() {
		It("Should contain desired ReplicaStatuses", func() {
			By("By creating an active launcher pod and active worker pods")

			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			jobName := "test-launcher-running-worker-running"

			var replicas int32 = 8
			mpiJob := newMPIJob(jobName, &replicas, 1, gpuResourceName, &startTime, &completionTime)
			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			launcher := reconciler.newLauncher(mpiJob, "kubectl-delivery", isGPULauncher(mpiJob))
			launcherKey := types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      launcher.GetName(),
			}
			Eventually(func() error {
				launcherCreated := &corev1.Pod{}
				if err := testK8sClient.Get(ctx, launcherKey, launcherCreated); err != nil {
					return err
				}
				launcherCreated.Status.Phase = corev1.PodRunning
				return testK8sClient.Status().Update(ctx, launcherCreated)
			}, testutil.Timeout, testutil.Interval).Should(BeNil())

			for i := 0; i < int(replicas); i++ {
				name := fmt.Sprintf("%s-%d", mpiJob.Name+workerSuffix, i)
				worker := reconciler.newWorker(mpiJob, name)
				workerKey := types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      worker.GetName(),
				}
				Eventually(func() error {
					workerCreated := &corev1.Pod{}
					if err := testK8sClient.Get(ctx, workerKey, workerCreated); err != nil {
						return err
					}
					workerCreated.Status.Phase = corev1.PodRunning
					return testK8sClient.Status().Update(ctx, workerCreated)
				}, testutil.Timeout, testutil.Interval).Should(BeNil())
			}

			key := types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      jobName,
			}
			launcherStatus := &common.ReplicaStatus{
				Active:    1,
				Succeeded: 0,
				Failed:    0,
			}
			workerStatus := &common.ReplicaStatus{
				Active:    8,
				Succeeded: 0,
				Failed:    0,
			}
			Eventually(func() bool {
				created := &kubeflowv1.MPIJob{}
				err := testK8sClient.Get(ctx, key, created)
				if err != nil {
					return false
				}
				return ReplicaStatusMatch(created.Status.ReplicaStatuses, kubeflowv1.MPIJobReplicaTypeLauncher,
					launcherStatus) && ReplicaStatusMatch(created.Status.ReplicaStatuses, kubeflowv1.MPIJobReplicaTypeWorker,
					workerStatus)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
		})
	})

	Context("Test MPIJob with Running worker Pods", func() {
		It("Should contain desired ReplicaStatuses and create a launcher pod", func() {
			By("By creating only active worker pods")

			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			jobName := "test-worker-running"

			var replicas int32 = 16
			mpiJob := newMPIJob(jobName, &replicas, 1, gpuResourceName, &startTime, &completionTime)
			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			for i := 0; i < int(replicas); i++ {
				name := fmt.Sprintf("%s-%d", mpiJob.Name+workerSuffix, i)
				worker := reconciler.newWorker(mpiJob, name)
				workerKey := types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      worker.GetName(),
				}
				Eventually(func() error {
					workerCreated := &corev1.Pod{}
					if err := testK8sClient.Get(ctx, workerKey, workerCreated); err != nil {
						return err
					}
					workerCreated.Status.Phase = corev1.PodRunning
					return testK8sClient.Status().Update(ctx, workerCreated)
				}, testutil.Timeout, testutil.Interval).Should(BeNil())
			}

			launcherKey := types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      mpiJob.Name + launcherSuffix,
			}
			launcher := &kubeflowv1.MPIJob{}
			Eventually(func() bool {
				err := testK8sClient.Get(ctx, launcherKey, launcher)
				return err != nil
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			key := types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      jobName,
			}
			launcherStatus := &common.ReplicaStatus{
				Active:    0,
				Succeeded: 0,
				Failed:    0,
			}
			workerStatus := &common.ReplicaStatus{
				Active:    16,
				Succeeded: 0,
				Failed:    0,
			}
			Eventually(func() bool {
				created := &kubeflowv1.MPIJob{}
				err := testK8sClient.Get(ctx, key, created)
				if err != nil {
					return false
				}
				return ReplicaStatusMatch(created.Status.ReplicaStatuses, kubeflowv1.MPIJobReplicaTypeLauncher,
					launcherStatus) && ReplicaStatusMatch(created.Status.ReplicaStatuses, kubeflowv1.MPIJobReplicaTypeWorker,
					workerStatus)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
		})
	})

	Context("MPIJob not found", func() {
		It("Should do nothing", func() {
			By("Calling Reconcile method")
			jobName := "test-not-exist"

			ctx := context.Background()

			req := ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      jobName,
			}}
			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).Should(BeNil())
		})
	})

	Context("MPI Job succeeds with predefined service account", func() {
		It("should run with the defined service account", func() {
			By("Calling Reconcile method")
			jobName := "test-sa-orphan"
			launcherSaName := "launcher-sa"

			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			mpiJob := newMPIJob(jobName, ptr.To[int32](64), 1, gpuResourceName, &startTime, &completionTime)
			mpiJob.Spec.MPIReplicaSpecs[kubeflowv1.MPIJobReplicaTypeLauncher].Template.Spec.ServiceAccountName = launcherSaName
			sa := newLauncherServiceAccount(mpiJob)
			sa.OwnerReferences = nil

			Expect(sa.Name).Should(Equal(launcherSaName))
			Expect(testK8sClient.Create(ctx, sa)).Should(Succeed())
			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			Eventually(func() error {
				req := ctrl.Request{NamespacedName: types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      mpiJob.GetName(),
				}}

				_, err := reconciler.Reconcile(ctx, req)

				if err != nil {
					return err
				}

				return nil
			}, testutil.Timeout, testutil.Interval).Should(BeNil())

			Eventually(func() string {
				launcherCreated := &corev1.Pod{}

				launcherKey := types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      mpiJob.Name + launcherSuffix,
				}

				if err := testK8sClient.Get(ctx, launcherKey, launcherCreated); err != nil {
					return ""
				}

				return launcherCreated.Spec.ServiceAccountName
			}, testutil.Timeout, testutil.Interval).Should(Equal(launcherSaName))
		})
	})

	Context("MPIJob with disabled RBAC management", func() {
		It("Should not create ServiceAccount, Role, or RoleBinding", func() {
			By("Setting DisableMPIRBACManagement to true")
			ctlrconfig.Config.DisableMPIRBACManagement = true
			defer func() {
				ctlrconfig.Config.DisableMPIRBACManagement = false
			}()

			By("Creating an MPIJob with a user-provided ServiceAccount")
			jobName := "test-disable-rbac"
			launcherSaName := "custom-launcher-sa"

			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			mpiJob := newMPIJob(jobName, ptr.To[int32](1), 1, gpuResourceName, &startTime, &completionTime)
			mpiJob.Spec.MPIReplicaSpecs[kubeflowv1.MPIJobReplicaTypeLauncher].Template.Spec.ServiceAccountName = launcherSaName

			// Pre-create the ServiceAccount since the operator won't create it
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      launcherSaName,
					Namespace: metav1.NamespaceDefault,
				},
			}
			Expect(testK8sClient.Create(ctx, sa)).Should(Succeed())
			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			By("Reconciling until the launcher pod is created")
			Eventually(func() error {
				req := ctrl.Request{NamespacedName: types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      mpiJob.GetName(),
				}}
				_, err := reconciler.Reconcile(ctx, req)
				return err
			}, testutil.Timeout, testutil.Interval).Should(BeNil())

			By("Verifying no Role was created")
			Eventually(func() bool {
				role := &rbacv1.Role{}
				err := testK8sClient.Get(ctx, types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      jobName + launcherSuffix,
				}, role)
				return errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Verifying no RoleBinding was created")
			Eventually(func() bool {
				rb := &rbacv1.RoleBinding{}
				err := testK8sClient.Get(ctx, types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      jobName + launcherSuffix,
				}, rb)
				return errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Verifying no operator-created ServiceAccount was created")
			Eventually(func() bool {
				operatorSa := &corev1.ServiceAccount{}
				err := testK8sClient.Get(ctx, types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      jobName + launcherSuffix,
				}, operatorSa)
				return errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Verifying ConfigMap was created")
			Eventually(func() error {
				cm := &corev1.ConfigMap{}
				return testK8sClient.Get(ctx, types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      jobName + configSuffix,
				}, cm)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Verifying kubectl delivery init container is present")
			Eventually(func() bool {
				launcher := &corev1.Pod{}
				err := testK8sClient.Get(ctx, types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      jobName + launcherSuffix,
				}, launcher)
				if err != nil {
					return false
				}
				for _, ic := range launcher.Spec.InitContainers {
					if ic.Name == kubectlDeliveryName {
						return true
					}
				}
				return false
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Verifying launcher pod uses the user-provided ServiceAccount")
			Eventually(func() string {
				launcher := &corev1.Pod{}
				err := testK8sClient.Get(ctx, types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      jobName + launcherSuffix,
				}, launcher)
				if err != nil {
					return ""
				}
				return launcher.Spec.ServiceAccountName
			}, testutil.Timeout, testutil.Interval).Should(Equal(launcherSaName))
		})
	})

	Context("SetupWithManager with disabled RBAC management", func() {
		It("Should not watch RBAC resources when flag is true", func() {
			By("Starting a second envtest instance for isolation")
			testEnv2 := &envtest.Environment{
				CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "manifests", "base", "crds")},
				ErrorIfCRDPathMissing: true,
			}
			cfg2, err := testEnv2.Start()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg2).NotTo(BeNil())
			defer testEnv2.Stop()

			By("Setting DisableMPIRBACManagement to true")
			ctlrconfig.Config.DisableMPIRBACManagement = true
			defer func() {
				ctlrconfig.Config.DisableMPIRBACManagement = false
			}()

			By("Creating a second manager with disabled RBAC management")
			mgr2, err := ctrl.NewManager(cfg2, ctrl.Options{
				Metrics: metricsserver.Options{
					BindAddress: "0",
				},
				Controller: ctrlconfig.Controller{
					SkipNameValidation: ptr.To(true),
				},
			})
			Expect(err).NotTo(HaveOccurred())

			gangSchedulingSetupFunc := ctrlcommon.GenNonGangSchedulerSetupFunc()
			reconciler2 := NewReconciler(mgr2, gangSchedulingSetupFunc)
			Expect(reconciler2.SetupWithManager(mgr2, 1)).NotTo(HaveOccurred())

			ctx2, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				defer GinkgoRecover()
				err := mgr2.Start(ctx2)
				Expect(err).ToNot(HaveOccurred(), "failed to run second manager")
			}()

			By("Creating a client for the second envtest")
			client2, err := client.New(cfg2, client.Options{Scheme: scheme.Scheme})
			Expect(err).NotTo(HaveOccurred())
			Expect(client2).NotTo(BeNil())

			By("Creating an MPIJob with a user-provided ServiceAccount")
			jobName := "test-watch-suppression"
			launcherSaName := "custom-watch-sa"

			mpiJob := newMPIJob(jobName, ptr.To[int32](1), 1, gpuResourceName, nil, nil)
			mpiJob.Spec.MPIReplicaSpecs[kubeflowv1.MPIJobReplicaTypeLauncher].Template.Spec.ServiceAccountName = launcherSaName

			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      launcherSaName,
					Namespace: metav1.NamespaceDefault,
				},
			}
			Expect(client2.Create(ctx2, sa)).Should(Succeed())
			Expect(client2.Create(ctx2, mpiJob)).Should(Succeed())

			launcherKey := types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      jobName + launcherSuffix,
			}

			By("Waiting for the launcher pod to be created")
			Eventually(func() error {
				return client2.Get(ctx2, launcherKey, &corev1.Pod{})
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Verifying no Role was created")
			Eventually(func() bool {
				role := &rbacv1.Role{}
				err := client2.Get(ctx2, types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      jobName + launcherSuffix,
				}, role)
				return errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Verifying no RoleBinding was created")
			Eventually(func() bool {
				rb := &rbacv1.RoleBinding{}
				err := client2.Get(ctx2, types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      jobName + launcherSuffix,
				}, rb)
				return errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Verifying no operator-created ServiceAccount was created")
			Eventually(func() bool {
				operatorSa := &corev1.ServiceAccount{}
				err := client2.Get(ctx2, types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      jobName + launcherSuffix,
				}, operatorSa)
				return errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Verifying ConfigMap was created")
			Eventually(func() error {
				return client2.Get(ctx2, types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      jobName + configSuffix,
				}, &corev1.ConfigMap{})
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Verifying kubectl delivery init container is present")
			Eventually(func() bool {
				launcher := &corev1.Pod{}
				err := client2.Get(ctx2, launcherKey, launcher)
				if err != nil {
					return false
				}
				for _, ic := range launcher.Spec.InitContainers {
					if ic.Name == kubectlDeliveryName {
						return true
					}
				}
				return false
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Verifying ConfigMap watch is active via ConfigMap update")
			cm := &corev1.ConfigMap{}
			Expect(client2.Get(ctx2, types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      jobName + configSuffix,
			}, cm)).Should(Succeed())

			// Corrupt the ConfigMap data
			cm.Data = map[string]string{"corrupted": "true"}
			Expect(client2.Update(ctx2, cm)).Should(Succeed())

			// The ConfigMap update should trigger a reconcile via the ConfigMap watch.
			// The reconciler should restore the original data.
			Eventually(func() bool {
				restoredCm := &corev1.ConfigMap{}
				err := client2.Get(ctx2, types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      jobName + configSuffix,
				}, restoredCm)
				if err != nil {
					return false
				}
				// Check that the corrupted key is gone and original data is restored
				_, hasCorrupt := restoredCm.Data["corrupted"]
				return !hasCorrupt && len(restoredCm.Data) > 0
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			// Note: RBAC watch suppression at the SetupWithManager level (the
			// `if !ctlrconfig.Config.DisableMPIRBACManagement` guard at line ~217)
			// is verified structurally by code inspection, not behaviorally. A
			// behavioral negative control is infeasible in envtest because:
			// 1. The Pod watch is always active (not gated by the flag), so pod
			//    state cannot be used as a signal for RBAC watch activity.
			// 2. No-op reconciles don't change MPIJob status in envtest (pods stay
			//    Pending, reflect.DeepEqual guard prevents status writes), so
			//    resourceVersion stability doesn't prove watch suppression.
			// The test verifies: SetupWithManager succeeds with flag=true, no RBAC
			// resources are created, ConfigMap/kubectl delivery remain unconditional,
			// ConfigMap and Pod watches remain active.
			By("Deleting launcher pod to verify reconciler is still functional")
			launcher := &corev1.Pod{}
			Expect(client2.Get(ctx2, launcherKey, launcher)).Should(Succeed())
			Expect(client2.Delete(ctx2, launcher)).Should(Succeed())

			By("Verifying launcher pod is recreated by the reconciler")
			Eventually(func() error {
				return client2.Get(ctx2, launcherKey, &corev1.Pod{})
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Verifying ConfigMap still exists after reconcile")
			Eventually(func() error {
				return client2.Get(ctx2, types.NamespacedName{
					Namespace: metav1.NamespaceDefault,
					Name:      jobName + configSuffix,
				}, &corev1.ConfigMap{})
			}, testutil.Timeout, testutil.Interval).Should(Succeed())
		})
	})

	Context("MPIJob with launcher Pod not controlled by itself", func() {
		It("Should return error", func() {
			By("Calling Reconcile method")
			jobName := "test-launcher-orphan"
			testKind := "Pod"

			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			mpiJob := newMPIJob(jobName, ptr.To[int32](64), 1, gpuResourceName, &startTime, &completionTime)

			launcher := reconciler.newLauncher(mpiJob, "kubectl-delivery", isGPULauncher(mpiJob))
			launcher.OwnerReferences = nil
			Expect(testK8sClient.Create(ctx, launcher)).Should(Succeed())

			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			req := ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      mpiJob.GetName(),
			}}
			expectedErr := fmt.Errorf(MessageResourceExists, launcher.Name, testKind)
			Eventually(func() error {
				_, err := reconciler.Reconcile(ctx, req)
				return err
			}, testutil.Timeout, testutil.Interval).Should(MatchError(expectedErr))
		})
	})

	Context("MPIJob with worker Pod not controlled by itself", func() {
		It("Should return error", func() {
			By("Calling Reconcile method")
			jobName := "test-worker-orphan"
			testKind := "Pod"

			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			mpiJob := newMPIJob(jobName, ptr.To[int32](1), 1, gpuResourceName, &startTime, &completionTime)

			for i := 0; i < 1; i++ {
				name := fmt.Sprintf("%s-%d", mpiJob.Name+workerSuffix, i)
				worker := reconciler.newWorker(mpiJob, name)
				worker.OwnerReferences = nil
				Expect(testK8sClient.Create(ctx, worker)).Should(Succeed())
			}

			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			req := ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      mpiJob.GetName(),
			}}
			expectedErr := fmt.Errorf(MessageResourceExists, fmt.Sprintf("%s-%d", mpiJob.Name+workerSuffix, 0), testKind)
			Eventually(func() error {
				_, err := reconciler.Reconcile(ctx, req)
				return err
			}, testutil.Timeout, testutil.Interval).Should(MatchError(expectedErr))
		})
	})

	Context("MPIJob with ConfigMap not controlled by itself", func() {
		It("Should return error", func() {
			By("Calling Reconcile method")
			jobName := "test-cm-orphan"
			testKind := "ConfigMap"

			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			mpiJob := newMPIJob(jobName, ptr.To[int32](64), 1, gpuResourceName, &startTime, &completionTime)

			cm := newConfigMap(mpiJob, 64, isGPULauncher(mpiJob))
			cm.OwnerReferences = nil
			Expect(testK8sClient.Create(ctx, cm)).Should(Succeed())

			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			req := ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      mpiJob.GetName(),
			}}
			expectedErr := fmt.Errorf(MessageResourceExists, cm.Name, testKind)
			Eventually(func() error {
				_, err := reconciler.Reconcile(ctx, req)
				return err
			}, testutil.Timeout, testutil.Interval).Should(MatchError(expectedErr))
		})
	})

	Context("MPIJob with Role not controlled by itself", func() {
		It("Should return error", func() {
			By("Calling Reconcile method")
			jobName := "test-role-orphan"
			testKind := "Role"

			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			mpiJob := newMPIJob(jobName, ptr.To[int32](64), 1, gpuResourceName, &startTime, &completionTime)

			role := newLauncherRole(mpiJob, 64)
			role.OwnerReferences = nil
			Expect(testK8sClient.Create(ctx, role)).Should(Succeed())

			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			req := ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      mpiJob.GetName(),
			}}
			expectedErr := fmt.Errorf(MessageResourceExists, role.Name, testKind)
			Eventually(func() error {
				_, err := reconciler.Reconcile(ctx, req)
				return err
			}, testutil.Timeout, testutil.Interval).Should(MatchError(expectedErr))
		})
	})

	Context("MPIJob with RoleBinding not controlled by itself", func() {
		It("Should return error", func() {
			By("Calling Reconcile method")
			jobName := "test-rb-orphan"
			testKind := "RoleBinding"

			ctx := context.Background()
			startTime := metav1.Now()
			completionTime := metav1.Now()

			mpiJob := newMPIJob(jobName, ptr.To[int32](64), 1, gpuResourceName, &startTime, &completionTime)

			rb := newLauncherRoleBinding(mpiJob)
			rb.OwnerReferences = nil
			Expect(testK8sClient.Create(ctx, rb)).Should(Succeed())

			Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

			req := ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: metav1.NamespaceDefault,
				Name:      mpiJob.GetName(),
			}}
			expectedErr := fmt.Errorf(MessageResourceExists, rb.Name, testKind)
			Eventually(func() error {
				_, err := reconciler.Reconcile(ctx, req)
				return err
			}, testutil.Timeout, testutil.Interval).Should(MatchError(expectedErr))
		})
	})

	Context("Test launcher's Intel MPI handling", func() {
		It("Should create a launcher job with Intel MPI env variables", func() {
			By("By creating MPIJobs with and without preset env variables")

			testCases := map[string]struct {
				envVariables         map[string]string
				expectedEnvVariables map[string]string
			}{
				"withoutIMPIValues": {
					envVariables: map[string]string{
						"X_MPI_HYDRA_BOOTSTRAP": "foo",
					},
					expectedEnvVariables: map[string]string{
						"I_MPI_HYDRA_BOOTSTRAP":      iMPIDefaultBootstrap,
						"I_MPI_HYDRA_BOOTSTRAP_EXEC": fmt.Sprintf("%s/%s", configMountPath, kubexecScriptName),
					},
				},
				"withIMPIBootstrap": {
					envVariables: map[string]string{
						"I_MPI_HYDRA_BOOTSTRAP": "RSH",
					},
					expectedEnvVariables: map[string]string{
						"I_MPI_HYDRA_BOOTSTRAP":      "RSH",
						"I_MPI_HYDRA_BOOTSTRAP_EXEC": fmt.Sprintf("%s/%s", configMountPath, kubexecScriptName),
					},
				},
				"withIMPIBootstrapExec": {
					envVariables: map[string]string{
						"I_MPI_HYDRA_BOOTSTRAP_EXEC": "/script.sh",
					},
					expectedEnvVariables: map[string]string{
						"I_MPI_HYDRA_BOOTSTRAP":      iMPIDefaultBootstrap,
						"I_MPI_HYDRA_BOOTSTRAP_EXEC": "/script.sh",
					},
				},
				"withIMPIBootstrapAndExec": {
					envVariables: map[string]string{
						"I_MPI_HYDRA_BOOTSTRAP":      "RSH",
						"I_MPI_HYDRA_BOOTSTRAP_EXEC": "/script.sh",
					},
					expectedEnvVariables: map[string]string{
						"I_MPI_HYDRA_BOOTSTRAP":      "RSH",
						"I_MPI_HYDRA_BOOTSTRAP_EXEC": "/script.sh",
					},
				},
			}

			for testName, testCase := range testCases {
				ctx := context.Background()
				startTime := metav1.Now()
				completionTime := metav1.Now()

				jobName := "test-launcher-creation-" + strings.ToLower(testName)

				mpiJob := newMPIJob(jobName, ptr.To[int32](1), 1, gpuResourceName, &startTime, &completionTime)
				Expect(testK8sClient.Create(ctx, mpiJob)).Should(Succeed())

				template := &mpiJob.Spec.MPIReplicaSpecs[kubeflowv1.MPIJobReplicaTypeLauncher].Template
				Expect(len(template.Spec.Containers) == 1).To(BeTrue())

				cont := &template.Spec.Containers[0]

				for k, v := range testCase.envVariables {
					cont.Env = append(cont.Env,
						corev1.EnvVar{
							Name:  k,
							Value: v,
						},
					)
				}

				launcher := reconciler.newLauncher(mpiJob, "kubectl-delivery", false)

				Expect(len(launcher.Spec.Containers) == 1).To(BeTrue())
				for expectedKey, expectedValue := range testCase.expectedEnvVariables {
					Expect(launcher.Spec.Containers[0].Env).Should(ContainElements(
						corev1.EnvVar{
							Name:  expectedKey,
							Value: expectedValue,
						}),
					)
				}
			}
		})
	})

	Context("When creating the MPIJob with the suspend semantics", func() {
		const name = "test-job"
		var (
			ns          *corev1.Namespace
			job         *kubeflowv1.MPIJob
			jobKey      types.NamespacedName
			launcherKey types.NamespacedName
			worker0Key  types.NamespacedName
			ctx         = context.Background()
		)
		BeforeEach(func() {
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "mpijob-test-",
				},
			}
			Expect(testK8sClient.Create(ctx, ns)).Should(Succeed())

			now := metav1.Now()
			job = newMPIJob(name, ptr.To[int32](1), 1, gpuResourceName, &now, &now)
			job.Namespace = ns.Name
			jobKey = client.ObjectKeyFromObject(job)
			launcherKey = types.NamespacedName{
				Name:      fmt.Sprintf("%s-launcher", name),
				Namespace: ns.Name,
			}
			worker0Key = types.NamespacedName{
				Name:      fmt.Sprintf("%s-worker-0", name),
				Namespace: ns.Name,
			}
		})
		AfterEach(func() {
			Expect(testK8sClient.Delete(ctx, job)).Should(Succeed())
			Expect(testK8sClient.Delete(ctx, ns)).Should(Succeed())
		})
		It("Shouldn't create resources if MPIJob is suspended", func() {
			By("By creating a new MPIJob with suspend=true")
			job.Spec.RunPolicy.Suspend = ptr.To(true)
			Expect(testK8sClient.Create(ctx, job)).Should(Succeed())

			created := &kubeflowv1.MPIJob{}
			launcherPod := &corev1.Pod{}
			workerPod := &corev1.Pod{}

			By("Checking created MPIJob")
			Eventually(func() bool {
				err := testK8sClient.Get(ctx, jobKey, created)
				return err == nil
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
			By("Checking created MPIJob has a nil startTime")
			Consistently(func() *metav1.Time {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				return created.Status.StartTime
			}, testutil.ConsistentDuration, testutil.Interval).Should(BeNil())

			By("Checking if the pods aren't created")
			Consistently(func() bool {
				errLauncherPod := testK8sClient.Get(ctx, launcherKey, launcherPod)
				errWorkerPod := testK8sClient.Get(ctx, worker0Key, workerPod)
				return errors.IsNotFound(errLauncherPod) && errors.IsNotFound(errWorkerPod)
			}, testutil.ConsistentDuration, testutil.Interval).Should(BeTrue())

			By("Checking if the MPIJob has suspended condition")
			Eventually(func() []kubeflowv1.JobCondition {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				return created.Status.Conditions
			}, testutil.ConsistentDuration, testutil.Interval).Should(BeComparableTo([]kubeflowv1.JobCondition{
				{
					Type:    kubeflowv1.JobCreated,
					Status:  corev1.ConditionTrue,
					Reason:  commonutil.NewReason(kubeflowv1.MPIJobKind, commonutil.JobCreatedReason),
					Message: fmt.Sprintf("MPIJob %s is created.", name),
				},
				{
					Type:    kubeflowv1.JobSuspended,
					Status:  corev1.ConditionTrue,
					Reason:  commonutil.NewReason(kubeflowv1.MPIJobKind, commonutil.JobSuspendedReason),
					Message: fmt.Sprintf("MPIJob %s is suspended.", name),
				},
			}, testutil.IgnoreJobConditionsTimes))
		})

		It("Should delete resources after MPIJob is suspended; Should resume MPIJob after MPIJob is unsuspended", func() {
			By("By creating a new MPIJob")
			Expect(testK8sClient.Create(ctx, job)).Should(Succeed())

			created := &kubeflowv1.MPIJob{}
			launcherPod := &corev1.Pod{}
			workerPod := &corev1.Pod{}

			// We'll need to retry getting this newly created MPIJob, given that creation may not immediately happen.
			By("Checking created MPIJob")
			Eventually(func() bool {
				err := testK8sClient.Get(ctx, jobKey, created)
				return err == nil
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			var startTimeBeforeSuspended *metav1.Time
			Eventually(func() *metav1.Time {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				startTimeBeforeSuspended = created.Status.StartTime
				return startTimeBeforeSuspended
			}, testutil.Timeout, testutil.Interval).ShouldNot(BeNil())

			By("Checking the created pods")
			Eventually(func() bool {
				errLauncher := testK8sClient.Get(ctx, launcherKey, launcherPod)
				errWorker := testK8sClient.Get(ctx, worker0Key, workerPod)
				return errLauncher == nil && errWorker == nil
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Updating the Pod's phase with Running")
			Eventually(func() error {
				Expect(testK8sClient.Get(ctx, launcherKey, launcherPod)).Should(Succeed())
				launcherPod.Status.Phase = corev1.PodRunning
				return testK8sClient.Status().Update(ctx, launcherPod)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())
			Eventually(func() error {
				Expect(testK8sClient.Get(ctx, worker0Key, workerPod)).Should(Succeed())
				workerPod.Status.Phase = corev1.PodRunning
				return testK8sClient.Status().Update(ctx, workerPod)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking the MPIJob's condition")
			Eventually(func() []kubeflowv1.JobCondition {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				return created.Status.Conditions
			}, testutil.Timeout, testutil.Interval).Should(BeComparableTo([]kubeflowv1.JobCondition{
				{
					Type:    kubeflowv1.JobCreated,
					Status:  corev1.ConditionTrue,
					Reason:  commonutil.NewReason(kubeflowv1.MPIJobKind, commonutil.JobCreatedReason),
					Message: fmt.Sprintf("MPIJob %s is created.", name),
				},
				{
					Type:    kubeflowv1.JobRunning,
					Status:  corev1.ConditionTrue,
					Reason:  commonutil.NewReason(kubeflowv1.MPIJobKind, commonutil.JobRunningReason),
					Message: fmt.Sprintf("MPIJob %s is running.", name),
				},
			}, testutil.IgnoreJobConditionsTimes))

			By("Updating the MPIJob with suspend=true")
			Eventually(func() error {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				created.Spec.RunPolicy.Suspend = ptr.To(true)
				return testK8sClient.Update(ctx, created)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking if the pods are removed")
			Eventually(func() bool {
				errLauncher := testK8sClient.Get(ctx, launcherKey, launcherPod)
				errWorker := testK8sClient.Get(ctx, worker0Key, workerPod)
				return errors.IsNotFound(errLauncher) && errors.IsNotFound(errWorker)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
			Consistently(func() bool {
				errLauncherPod := testK8sClient.Get(ctx, launcherKey, launcherPod)
				errWorkerPod := testK8sClient.Get(ctx, worker0Key, workerPod)
				return errors.IsNotFound(errLauncherPod) && errors.IsNotFound(errWorkerPod)
			}, testutil.ConsistentDuration, testutil.Interval).Should(BeTrue())

			By("Checking if the MPIJob has a suspended condition")
			Eventually(func() bool {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				return created.Status.ReplicaStatuses[kubeflowv1.MPIJobReplicaTypeLauncher].Active == 0 &&
					created.Status.ReplicaStatuses[kubeflowv1.MPIJobReplicaTypeWorker].Active == 0 &&
					created.Status.StartTime.Equal(startTimeBeforeSuspended)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
			Consistently(func() bool {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				return created.Status.ReplicaStatuses[kubeflowv1.MPIJobReplicaTypeLauncher].Active == 0 &&
					created.Status.ReplicaStatuses[kubeflowv1.MPIJobReplicaTypeWorker].Active == 0 &&
					created.Status.StartTime.Equal(startTimeBeforeSuspended)
			}, testutil.ConsistentDuration, testutil.Interval).Should(BeTrue())
			Expect(created.Status.Conditions).Should(BeComparableTo([]kubeflowv1.JobCondition{
				{
					Type:    kubeflowv1.JobCreated,
					Status:  corev1.ConditionTrue,
					Reason:  commonutil.NewReason(kubeflowv1.MPIJobKind, commonutil.JobCreatedReason),
					Message: fmt.Sprintf("MPIJob %s is created.", name),
				},
				{
					Type:    kubeflowv1.JobRunning,
					Status:  corev1.ConditionFalse,
					Reason:  commonutil.NewReason(kubeflowv1.MPIJobKind, commonutil.JobSuspendedReason),
					Message: fmt.Sprintf("MPIJob %s is suspended.", name),
				},
				{
					Type:    kubeflowv1.JobSuspended,
					Reason:  commonutil.NewReason(kubeflowv1.MPIJobKind, commonutil.JobSuspendedReason),
					Message: fmt.Sprintf("MPIJob %s is suspended.", name),
					Status:  corev1.ConditionTrue,
				},
			}, testutil.IgnoreJobConditionsTimes))

			By("Unsuspending the MPIJob")
			Eventually(func() error {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				created.Spec.RunPolicy.Suspend = ptr.To(false)
				return testK8sClient.Update(ctx, created)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())
			Eventually(func() *metav1.Time {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				return created.Status.StartTime
			}, testutil.Timeout, testutil.Interval).ShouldNot(BeNil())

			By("Check if the pods are created")
			Eventually(func() error {
				return testK8sClient.Get(ctx, launcherKey, launcherPod)
			}, testutil.Timeout, testutil.Interval).Should(BeNil())
			Eventually(func() error {
				return testK8sClient.Get(ctx, worker0Key, workerPod)
			}, testutil.Timeout, testutil.Interval).Should(BeNil())

			By("Updating Pod's condition with Running")
			Eventually(func() error {
				Expect(testK8sClient.Get(ctx, launcherKey, launcherPod)).Should(Succeed())
				launcherPod.Status.Phase = corev1.PodRunning
				return testK8sClient.Status().Update(ctx, launcherPod)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())
			Eventually(func() error {
				Expect(testK8sClient.Get(ctx, worker0Key, workerPod)).Should(Succeed())
				workerPod.Status.Phase = corev1.PodRunning
				return testK8sClient.Status().Update(ctx, workerPod)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking if the MPIJob has resumed conditions")
			Eventually(func() []kubeflowv1.JobCondition {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				return created.Status.Conditions
			}, testutil.Timeout, testutil.Interval).Should(BeComparableTo([]kubeflowv1.JobCondition{
				{
					Type:    kubeflowv1.JobCreated,
					Status:  corev1.ConditionTrue,
					Reason:  commonutil.NewReason(kubeflowv1.MPIJobKind, commonutil.JobCreatedReason),
					Message: fmt.Sprintf("MPIJob %s is created.", name),
				},
				{
					Type:    kubeflowv1.JobSuspended,
					Reason:  commonutil.NewReason(kubeflowv1.MPIJobKind, commonutil.JobResumedReason),
					Message: fmt.Sprintf("MPIJob %s is resumed.", name),
					Status:  corev1.ConditionFalse,
				},
				{
					Type:    kubeflowv1.JobRunning,
					Status:  corev1.ConditionTrue,
					Reason:  commonutil.NewReason(kubeflowv1.MPIJobKind, commonutil.JobRunningReason),
					Message: fmt.Sprintf("MPIJob %s is running.", name),
				},
			}, testutil.IgnoreJobConditionsTimes))

			By("Checking if the startTime is updated")
			Expect(created.Status.StartTime).ShouldNot(Equal(startTimeBeforeSuspended))
		})

		It("Should not reconcile a job while managed by external controller", func() {
			By("Creating a MPIJob managed by external controller")
			job.Spec.RunPolicy = kubeflowv1.RunPolicy{
				ManagedBy: ptr.To(kubeflowv1.MultiKueueController),
			}
			job.Spec.RunPolicy.Suspend = ptr.To(true)
			Expect(testK8sClient.Create(ctx, job)).Should(Succeed())

			created := &kubeflowv1.MPIJob{}
			By("Checking created MPIJob")
			Eventually(func() bool {
				err := testK8sClient.Get(ctx, jobKey, created)
				return err == nil
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Checking created MPIJob has a nil startTime")
			Consistently(func() *metav1.Time {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				return created.Status.StartTime
			}, testutil.ConsistentDuration, testutil.Interval).Should(BeNil())

			By("Checking if the pods and services aren't created")
			Consistently(func() bool {
				launcherPod := &corev1.Pod{}
				workerPod := &corev1.Pod{}
				launcherSvc := &corev1.Service{}
				workerSvc := &corev1.Service{}
				errMasterPod := testK8sClient.Get(ctx, launcherKey, launcherPod)
				errWorkerPod := testK8sClient.Get(ctx, worker0Key, workerPod)
				errMasterSvc := testK8sClient.Get(ctx, launcherKey, launcherSvc)
				errWorkerSvc := testK8sClient.Get(ctx, worker0Key, workerSvc)
				return errors.IsNotFound(errMasterPod) && errors.IsNotFound(errWorkerPod) &&
					errors.IsNotFound(errMasterSvc) && errors.IsNotFound(errWorkerSvc)
			}, testutil.ConsistentDuration, testutil.Interval).Should(BeTrue(), "pods and services should be created by external controller (here not existent)")

			By("Checking if the MPIJob status was not updated")
			Eventually(func() []kubeflowv1.JobCondition {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				return created.Status.Conditions
			}, testutil.Timeout, testutil.Interval).Should(BeComparableTo([]kubeflowv1.JobCondition(nil)))

			By("Unsuspending the MPIJob")
			Eventually(func() error {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				created.Spec.RunPolicy.Suspend = ptr.To(false)
				return testK8sClient.Update(ctx, created)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking created MPIJob still has a nil startTime")
			Consistently(func() *metav1.Time {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				return created.Status.StartTime
			}, testutil.ConsistentDuration, testutil.Interval).Should(BeNil())

			By("Checking if the MPIJob status was not updated, even after unsuspending")
			Eventually(func() []kubeflowv1.JobCondition {
				Expect(testK8sClient.Get(ctx, jobKey, created)).Should(Succeed())
				return created.Status.Conditions
			}, testutil.Timeout, testutil.Interval).Should(BeComparableTo([]kubeflowv1.JobCondition(nil)))
		})
	})
})

func ReplicaStatusMatch(replicaStatuses map[common.ReplicaType]*common.ReplicaStatus,
	replicaType common.ReplicaType, status *common.ReplicaStatus) bool {

	result := true

	if replicaStatuses == nil {
		return false
	}
	if val, exist := replicaStatuses[replicaType]; !exist {
		return false
	} else {
		result = result && (val.Active == status.Active)
		result = result && (val.Succeeded == status.Succeeded)
		result = result && (val.Failed == status.Failed)
	}

	return result
}
