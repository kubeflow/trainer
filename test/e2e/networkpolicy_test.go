/*
Copyright The Kubeflow Authors.

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

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	jobset "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

var _ = ginkgo.Describe("TrainJob network isolation", func() {
	ginkgo.It("enforces peer, cache, DNS and administrator rules and repairs active policies", func() {
		if os.Getenv("TRAINER_NETWORK_POLICY_E2E") != "true" {
			ginkgo.Skip("requires network policies enabled with testdata/network-policy-values.yaml")
		}
		var namespaces []*corev1.Namespace
		for range 2 {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "network-policy-"}}
			gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
			namespaces = append(namespaces, ns)
			ginkgo.DeferCleanup(func() { gomega.Expect(k8sClient.Delete(ctx, ns)).To(gomega.Succeed()) })
			runtime := &trainer.TrainingRuntime{
				ObjectMeta: metav1.ObjectMeta{Name: "http", Namespace: ns.Name},
				Spec: trainer.TrainingRuntimeSpec{Template: trainer.JobSetTemplateSpec{Spec: jobset.JobSetSpec{
					ReplicatedJobs: []jobset.ReplicatedJob{{Name: "node", Template: batchv1.JobTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{constants.LabelTrainJobAncestor: constants.AncestorTrainer}},
						Spec:       batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: networkTestPodSpec()}},
					}}},
				}}},
			}
			gomega.Expect(k8sClient.Create(ctx, runtime)).To(gomega.Succeed())
		}
		jobs := []*trainer.TrainJob{}
		for _, placement := range []struct{ namespace, name string }{
			{namespaces[0].Name, "alpha"}, {namespaces[0].Name, "beta"}, {namespaces[1].Name, "alpha"},
		} {
			job := testingutil.MakeTrainJobWrapper(placement.namespace, placement.name).
				RuntimeRef(trainer.GroupVersion.WithKind(trainer.TrainingRuntimeKind), "http").
				Trainer(&trainer.Trainer{NumNodes: ptr.To(int32(2))}).Obj()
			gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.Succeed())
			jobs = append(jobs, job)
		}
		workers := make([][]corev1.Pod, 3)
		for i, job := range jobs {
			gomega.Eventually(func(g gomega.Gomega) {
				pods := &corev1.PodList{}
				g.Expect(k8sClient.List(ctx, pods, client.InNamespace(job.Namespace), client.MatchingLabels{jobset.JobSetNameKey: job.Name})).To(gomega.Succeed())
				g.Expect(pods.Items).To(gomega.HaveLen(2))
				for _, pod := range pods.Items {
					g.Expect(pod.Status.Phase).To(gomega.Equal(corev1.PodRunning))
					g.Expect(pod.Status.PodIP).NotTo(gomega.BeEmpty())
				}
				workers[i] = pods.Items
			}, 3*time.Minute, time.Second).Should(gomega.Succeed())
		}
		ginkgo.By("allow API egress without an administrator exception")
		gomega.Eventually(func(g gomega.Gomega) {
			output, err := networkTestExec(&workers[0][0], "sh", "-c",
				`wget -T 5 -qO- --no-check-certificate --header="Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" https://kubernetes.default.svc/version`)
			g.Expect(err).NotTo(gomega.HaveOccurred(), output)
			g.Expect(output).To(gomega.ContainSubstring("gitVersion"))
		}, 30*time.Second, time.Second).Should(gomega.Succeed())
		// Synthetic cache labels require LWS admission disabled; see the installation guide.
		cache := networkTestPod(namespaces[0].Name, "cache-alpha", map[string]string{"leaderworkerset.sigs.k8s.io/name": "alpha-cache"})
		otherCache := networkTestPod(namespaces[0].Name, "cache-beta", map[string]string{"leaderworkerset.sigs.k8s.io/name": "beta-cache"})
		outsider := networkTestPod(namespaces[0].Name, "outsider", nil)
		approved := networkTestPod(namespaces[0].Name, "approved", map[string]string{"network-policy-test-allowed": "true"})
		for _, pod := range []*corev1.Pod{cache, otherCache, outsider, approved} {
			gomega.Expect(k8sClient.Create(ctx, pod)).To(gomega.Succeed())
			gomega.Eventually(func(g gomega.Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(gomega.Succeed())
				g.Expect(pod.Status.Phase).To(gomega.Equal(corev1.PodRunning))
				g.Expect(pod.Status.PodIP).NotTo(gomega.BeEmpty())
			}, time.Minute, time.Second).Should(gomega.Succeed())
		}
		a, b, c := workers[0], workers[1], workers[2]
		for _, pair := range [][2]*corev1.Pod{{&a[0], &a[1]}, {&b[0], &b[1]}, {&c[0], &c[1]}, {&a[0], cache}, {cache, &a[0]}, {&b[0], otherCache}, {&a[0], approved}, {approved, &a[0]}} {
			source, target := pair[0], pair[1]
			ginkgo.By(fmt.Sprintf("allow %s/%s -> %s/%s", source.Namespace, source.Name, target.Namespace, target.Name))
			gomega.Eventually(func() error { return networkTestRequest(source, target, true) }, 30*time.Second, time.Second).Should(gomega.Succeed())
		}
		ginkgo.By("allow DNS egress")
		output, err := networkTestExec(&a[0], "nslookup", "kubernetes.default.svc.cluster.local")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), output)
		for _, pair := range [][2]*corev1.Pod{{&a[0], &b[0]}, {&b[0], &a[0]}, {&a[0], &c[0]}, {&c[0], &a[0]}, {outsider, &a[0]}, {&a[0], outsider}, {&a[0], otherCache}, {otherCache, &a[0]}, {cache, otherCache}} {
			source, target := pair[0], pair[1]
			ginkgo.By(fmt.Sprintf("deny %s/%s -> %s/%s", source.Namespace, source.Name, target.Namespace, target.Name))
			gomega.Eventually(func() error { return networkTestRequest(source, target, false) }, 30*time.Second, time.Second).Should(gomega.Succeed())
		}
		policy := &networkingv1.NetworkPolicy{}
		key := client.ObjectKey{Namespace: jobs[0].Namespace, Name: "trainjob-" + string(jobs[0].UID)}
		gomega.Expect(k8sClient.Get(ctx, key, policy)).To(gomega.Succeed())
		ginkgo.By("repair an administrator rule edit on an active policy")
		policy.Spec.Ingress = append(policy.Spec.Ingress, networkingv1.NetworkPolicyIngressRule{})
		gomega.Expect(k8sClient.Update(ctx, policy)).To(gomega.Succeed())
		gomega.Eventually(func(g gomega.Gomega) {
			g.Expect(k8sClient.Get(ctx, key, policy)).To(gomega.Succeed())
			g.Expect(policy.Spec.Ingress).To(gomega.HaveLen(2))
		}, 30*time.Second, time.Second).Should(gomega.Succeed())
		ginkgo.By("recreate a deleted policy")
		oldUID := policy.UID
		gomega.Expect(k8sClient.Delete(ctx, policy)).To(gomega.Succeed())
		gomega.Eventually(func(g gomega.Gomega) {
			g.Expect(k8sClient.Get(ctx, key, policy)).To(gomega.Succeed())
			g.Expect(policy.UID).NotTo(gomega.Equal(oldUID))
		}, 30*time.Second, time.Second).Should(gomega.Succeed())
		gomega.Eventually(func() error { return networkTestRequest(&b[0], &a[0], false) }, 30*time.Second, time.Second).Should(gomega.Succeed())
		ginkgo.By("garbage collect policies when their TrainJob is deleted")
		gomega.Expect(k8sClient.Delete(ctx, jobs[0])).To(gomega.Succeed())
		for _, name := range []string{key.Name, "trainjob-cache-" + string(jobs[0].UID)} {
			gomega.Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: name}, &networkingv1.NetworkPolicy{}))
			}, time.Minute, time.Second).Should(gomega.BeTrue())
		}
	})
})

func networkTestPodSpec() corev1.PodSpec {
	return corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{
		Name: "node", Image: "busybox:1.37.0", Command: []string{"sh", "-c", "mkdir -p /www; echo ok >/www/index.html; exec httpd -f -p 8080 -h /www"},
	}}}
}

func networkTestPod(namespace, name string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: labels}, Spec: networkTestPodSpec()}
}

func networkTestRequest(source, target *corev1.Pod, allowed bool) error {
	output, err := networkTestExec(source, "wget", "-T", "2", "-qO-", "http://"+target.Status.PodIP+":8080")
	if allowed {
		if err != nil {
			return fmt.Errorf("request failed: %w: %s", err, output)
		}
		if strings.TrimSpace(output) != "ok" {
			return fmt.Errorf("unexpected HTTP body: %s", output)
		}
	} else if err == nil || !strings.Contains(output, "timed out") {
		return fmt.Errorf("expected network timeout, got %v: %s", err, output)
	}
	return nil
}

func networkTestExec(pod *corev1.Pod, command ...string) (string, error) {
	cfg := config.GetConfigOrDie()
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", err
	}
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name(pod.Name).Namespace(pod.Namespace).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{Command: command, Container: "node", Stdout: true, Stderr: true}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return "", err
	}
	var output, stderr bytes.Buffer
	execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = executor.StreamWithContext(execCtx, remotecommand.StreamOptions{Stdout: &output, Stderr: &stderr})
	return output.String() + stderr.String(), err
}
