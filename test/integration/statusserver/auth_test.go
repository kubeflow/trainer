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

package statusserver

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubeflow/trainer/v2/pkg/statusserver"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
	"github.com/kubeflow/trainer/v2/test/integration/framework"
	"github.com/kubeflow/trainer/v2/test/util"
)

var _ = ginkgo.Describe("StatusServer TokenReview authorizer", ginkgo.Ordered, func() {
	const (
		serviceAccountName = "trainer"
		trainJobName       = "test-job"
	)

	var (
		clientset  kubernetes.Interface
		authorizer statusserver.TokenAuthorizer
		ns         *corev1.Namespace
	)

	ginkgo.BeforeAll(func() {
		fwk = &framework.Framework{}
		cfg = fwk.Init()
		ctx, k8sClient = fwk.RunManager(cfg, false)
		clientset = kubernetes.NewForConfigOrDie(cfg)
		authorizer = statusserver.NewTokenReviewAuthorizer(k8sClient)
		gomega.Expect(authorizer.Init(ctx)).To(gomega.Succeed())
	})

	ginkgo.AfterAll(func() {
		fwk.Teardown()
	})

	createNamespace := func() *corev1.Namespace {
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "status-auth-"},
		}
		gomega.Expect(k8sClient.Create(ctx, namespace)).To(gomega.Succeed())
		gomega.Expect(k8sClient.Create(ctx, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace.Name},
		})).To(gomega.Succeed())
		return namespace
	}

	// requestToken issues a service account token in namespace for the given audiences,
	// optionally bound to a pod, like the projected token mounted into the trainer pods.
	requestToken := func(namespace string, audiences []string, boundPod *corev1.Pod) string {
		tokenRequest := &authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{Audiences: audiences},
		}
		if boundPod != nil {
			tokenRequest.Spec.BoundObjectRef = &authenticationv1.BoundObjectReference{
				Kind:       "Pod",
				APIVersion: "v1",
				Name:       boundPod.Name,
				UID:        boundPod.UID,
			}
		}
		gotTokenRequest, err := clientset.CoreV1().ServiceAccounts(namespace).
			CreateToken(ctx, serviceAccountName, tokenRequest, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		return gotTokenRequest.Status.Token
	}

	ginkgo.BeforeEach(func() {
		ns = createNamespace()
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(k8sClient.Delete(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.It("should authorize a token issued for the TrainJob audience", func() {
		token := requestToken(ns.Name, []string{statusserver.TokenAudience(ns.Name, trainJobName)}, nil)

		authorized, err := authorizer.Authorize(ctx, "Bearer "+token, ns.Name, trainJobName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(authorized).To(gomega.BeTrue())
	})

	ginkgo.It("should not authorize a token issued for a different TrainJob", func() {
		token := requestToken(ns.Name, []string{statusserver.TokenAudience(ns.Name, "other-job")}, nil)

		authorized, err := authorizer.Authorize(ctx, "Bearer "+token, ns.Name, trainJobName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(authorized).To(gomega.BeFalse())
	})

	ginkgo.It("should not authorize a token issued without the TrainJob audience", func() {
		token := requestToken(ns.Name, nil, nil)

		authorized, err := authorizer.Authorize(ctx, "Bearer "+token, ns.Name, trainJobName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(authorized).To(gomega.BeFalse())
	})

	ginkgo.It("should not authorize a token of a service account in a different namespace", func() {
		otherNs := createNamespace()
		ginkgo.DeferCleanup(func() {
			gomega.Expect(k8sClient.Delete(ctx, otherNs)).To(gomega.Succeed())
		})
		token := requestToken(otherNs.Name, []string{statusserver.TokenAudience(ns.Name, trainJobName)}, nil)

		authorized, err := authorizer.Authorize(ctx, "Bearer "+token, ns.Name, trainJobName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(authorized).To(gomega.BeFalse())
	})

	ginkgo.It("should not authorize a token that is not issued by the cluster", func() {
		authorized, err := authorizer.Authorize(ctx, "Bearer not-a-jwt", ns.Name, trainJobName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(authorized).To(gomega.BeFalse())
	})

	ginkgo.It("should stop authorizing a token once its bound pod is deleted", func() {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "trainer-node", Namespace: ns.Name},
			Spec: corev1.PodSpec{
				ServiceAccountName: serviceAccountName,
				Containers:         []corev1.Container{{Name: "node", Image: "trainer"}},
			},
		}
		gomega.Expect(k8sClient.Create(ctx, pod)).To(gomega.Succeed())
		token := requestToken(ns.Name, []string{statusserver.TokenAudience(ns.Name, trainJobName)}, pod)

		authorized, err := authorizer.Authorize(ctx, "Bearer "+token, ns.Name, trainJobName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(authorized).To(gomega.BeTrue())

		gomega.Expect(k8sClient.Delete(ctx, pod)).To(gomega.Succeed())
		gomega.Eventually(func(g gomega.Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), &corev1.Pod{})).To(testingutil.BeNotFoundError())
			authorized, err := authorizer.Authorize(ctx, "Bearer "+token, ns.Name, trainJobName)
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(authorized).To(gomega.BeFalse())
		}, util.Timeout, util.Interval).Should(gomega.Succeed())
	})
})
