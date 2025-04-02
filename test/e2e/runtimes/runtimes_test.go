package runtimes

import (
	"context"
	"fmt"
	"testing"

	kubeflowv2 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v2alpha1"
	"github.com/kubeflow/training-operator/pkg/client/clientset/versioned"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func TestCreateTrainingRuntime(t *testing.T) {
	namespace := "test-namespace-1"
	runtime := &kubeflowv2.TrainingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-runtime",
			Namespace: namespace,
		},
		Spec: kubeflowv2.TrainingRuntimeSpec{},
	}

	trainingClient, clientset := getTrainingClient(t, namespace)
	defer clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	defer trainingClient.KubeflowV2alpha1().TrainingRuntimes(namespace).Delete(context.TODO(), "test-runtime", metav1.DeleteOptions{})

	_, err := trainingClient.KubeflowV2alpha1().TrainingRuntimes(namespace).Create(context.TODO(), runtime, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TrainingRuntime: %v", err)
	}
}

func TestTrainingRuntimeBehavior(t *testing.T) {
	namespace := "test-namespace-2"
	runtime := &kubeflowv2.TrainingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-runtime",
			Namespace: namespace,
		},
		Spec: kubeflowv2.TrainingRuntimeSpec{},
	}

	trainingClient, clientset := getTrainingClient(t, namespace)
	defer clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	defer trainingClient.KubeflowV2alpha1().TrainingRuntimes(namespace).Delete(context.TODO(), "test-runtime", metav1.DeleteOptions{})

	_, err := trainingClient.KubeflowV2alpha1().TrainingRuntimes(namespace).Create(context.TODO(), runtime, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TrainingRuntime: %v", err)
	}

	job := &kubeflowv2.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: namespace,
		},
		Spec: kubeflowv2.TrainJobSpec{
			RuntimeRef: kubeflowv2.RuntimeRef{
				Name: "test-runtime",
			},
		},
	}
	_, err = trainingClient.KubeflowV2alpha1().TrainJobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TrainingJob: %v", err)
	}

}

func TestDeleteTrainingRuntime(t *testing.T) {
	namespace := "test-namespace-3"
	runtime := &kubeflowv2.TrainingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-runtime",
			Namespace: namespace,
		},
		Spec: kubeflowv2.TrainingRuntimeSpec{},
	}

	trainingClient, clientset := getTrainingClient(t, namespace)
	defer clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	defer trainingClient.KubeflowV2alpha1().TrainingRuntimes(namespace).Delete(context.TODO(), "test-runtime", metav1.DeleteOptions{})

	_, err := trainingClient.KubeflowV2alpha1().TrainingRuntimes(namespace).Create(context.TODO(), runtime, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TrainingRuntime: %v", err)
	}

	err = trainingClient.KubeflowV2alpha1().TrainingRuntimes(namespace).Delete(context.TODO(), "test-runtime", metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete TrainingRuntime: %v", err)
	}

	_, err = trainingClient.KubeflowV2alpha1().TrainingRuntimes(namespace).Get(context.TODO(), "test-runtime", metav1.GetOptions{})
	if !errors.IsNotFound(err) {
		t.Fatalf("TrainingRuntime was not deleted: %v", err)
	}
}

func TestInvalidTrainingRuntime(t *testing.T) {
	namespace := "test-namespace-4"
	runtime := &kubeflowv2.TrainingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-runtime",
			Namespace: "invalid-namespace",
		},
		Spec: kubeflowv2.TrainingRuntimeSpec{
			Template: kubeflowv2.JobSetTemplateSpec{
				Spec: jobsetv1alpha2.JobSetSpec{},
			},
		},
	}

	trainingClient, clientset := getTrainingClient(t, namespace)
	defer clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	defer trainingClient.KubeflowV2alpha1().TrainingRuntimes(namespace).Delete(context.TODO(), "test-runtime", metav1.DeleteOptions{})

	_, err := trainingClient.KubeflowV2alpha1().TrainingRuntimes(namespace).Create(context.TODO(), runtime, metav1.CreateOptions{})
	if err == nil {
		t.Fatal("Expected error for invalid TrainingRuntime, but got none")
	}
}

func TestScalingTrainingRuntimes(t *testing.T) {
	namespace := "test-namespace-5"
	for i := 0; i < 10; i++ {
		runtime := &kubeflowv2.TrainingRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-runtime-%d", i),
				Namespace: namespace,
			},
			Spec: kubeflowv2.TrainingRuntimeSpec{},
		}

		trainingClient, clientset := getTrainingClient(t, namespace)
		defer clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
		defer trainingClient.KubeflowV2alpha1().TrainingRuntimes(namespace).Delete(context.TODO(), "test-runtime", metav1.DeleteOptions{})

		_, err := trainingClient.KubeflowV2alpha1().TrainingRuntimes(namespace).Create(context.TODO(), runtime, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create TrainingRuntime %d: %v", i, err)
		}
	}
}

func getTrainingClient(t *testing.T, namespace string) (*versioned.Clientset, *kubernetes.Clientset) {
	var config *rest.Config
	var err error

	// Try in-cluster config first
	config, err = rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			t.Fatalf("Failed to load kubeconfig: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Create a test namespace
	_, err = clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create namespace: %v", err)
		}
	}

	trainingClient, err := versioned.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create TrainingRuntimes client: %v", err)
	}
	return trainingClient, clientset

}
