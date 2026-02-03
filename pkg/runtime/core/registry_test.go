package core

import "testing"

func TestNewRuntimeRegistry(t *testing.T) {
	registry := NewRuntimeRegistry()

	if registry == nil {
		t.Fatalf("expected registry to be non-nil")
	}

	// Expect exactly two registered runtimes
	if len(registry) != 2 {
		t.Fatalf("expected 2 runtime registrars, got %d", len(registry))
	}

	// TrainingRuntime must exist
	tr, ok := registry[TrainingRuntimeGroupKind]
	if !ok {
		t.Fatalf("expected TrainingRuntimeGroupKind to be registered")
	}
	if tr.factory == nil {
		t.Fatalf("expected TrainingRuntime factory to be non-nil")
	}

	// ClusterTrainingRuntime must exist
	ctr, ok := registry[ClusterTrainingRuntimeGroupKind]
	if !ok {
		t.Fatalf("expected ClusterTrainingRuntimeGroupKind to be registered")
	}
	if ctr.factory == nil {
		t.Fatalf("expected ClusterTrainingRuntime factory to be non-nil")
	}

	// ClusterTrainingRuntime should depend on TrainingRuntime
	if len(ctr.dependencies) != 1 || ctr.dependencies[0] != TrainingRuntimeGroupKind {
		t.Fatalf(
			"expected ClusterTrainingRuntime to depend on %q, got %v",
			TrainingRuntimeGroupKind,
			ctr.dependencies,
		)
	}
}
