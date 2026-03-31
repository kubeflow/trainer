package statusserver

import "testing"

func TestTokenAudience(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		trainJob  string
		expected  string
	}{
		{
			name:      "default namespace",
			namespace: "default",
			trainJob:  "mnist",
			expected:  "trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/mnist/status",
		},
		{
			name:      "kubeflow namespace",
			namespace: "kubeflow",
			trainJob:  "llm-job",
			expected:  "trainer.kubeflow.org/v1alpha1/namespaces/kubeflow/trainjobs/llm-job/status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TokenAudience(tt.namespace, tt.trainJob)
			if got != tt.expected {
				t.Fatalf("TokenAudience(%q, %q) = %q, want %q", tt.namespace, tt.trainJob, got, tt.expected)
			}
		})
	}
}

func TestStatusUrl(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		trainJob  string
		expected  string
	}{
		{
			name:      "default namespace",
			namespace: "default",
			trainJob:  "mnist",
			expected:  "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/mnist/status",
		},
		{
			name:      "kubeflow namespace",
			namespace: "kubeflow",
			trainJob:  "llm-job",
			expected:  "/apis/trainer.kubeflow.org/v1alpha1/namespaces/kubeflow/trainjobs/llm-job/status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatusUrl(tt.namespace, tt.trainJob)
			if got != tt.expected {
				t.Fatalf("StatusUrl(%q, %q) = %q, want %q", tt.namespace, tt.trainJob, got, tt.expected)
			}
		})
	}
}