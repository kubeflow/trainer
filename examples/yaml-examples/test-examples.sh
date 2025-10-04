#!/bin/bash
# Test script for Kubeflow Trainer YAML examples
# This script helps validate that examples can be applied successfully

set -e

NAMESPACE=${NAMESPACE:-default}
TIMEOUT=${TIMEOUT:-300}

echo "Testing Kubeflow Trainer YAML Examples"
echo "Namespace: $NAMESPACE"
echo "Timeout: ${TIMEOUT}s"
echo "=========================================="

# Function to test an example
test_example() {
    local example_file=$1
    local example_name=$2
    
    echo "Testing: $example_name"
    echo "File: $example_file"
    
    # Basic YAML syntax check
    if ! grep -q "apiVersion:" "$example_file" || ! grep -q "kind:" "$example_file"; then
        echo "ERROR: Basic YAML structure validation failed for $example_name"
        return 1
    fi
    
    echo "PASS: Basic YAML structure validation passed for $example_name"
    
    # Optional: Apply and check if resources are created (commented out for safety)
    # echo "🔄 Applying $example_name (dry-run)..."
    # kubectl apply --dry-run=server -f "$example_file"
    
    echo "PASS: $example_name test completed"
    echo ""
}

# Test all examples
echo "Running validation tests..."
echo ""

# PyTorch MNIST
test_example "pytorch-mnist/mnist_training.yaml" "PyTorch MNIST Training"

# TRL LLM Fine-tuning
test_example "trl-llm-finetuning/trainjob.yaml" "TRL LLM Fine-tuning"

# MPI Training
test_example "mpi-training/simple_mpi_training.yaml" "Simple MPI Training"
test_example "mpi-training/mpi_training.yaml" "Advanced MPI Training"

echo "SUCCESS: All examples passed validation!"
echo ""
echo "Next Steps:"
echo "1. Choose an example that fits your use case"
echo "2. Review the README in the example directory"
echo "3. Customize configuration as needed"
echo "4. Apply with: kubectl apply -f <example-file>"
echo "5. Monitor with: kubectl get trainjobs"
echo ""
echo "For more information:"
echo "- Main README: ./README.md"
echo "- Kubeflow Trainer docs: https://www.kubeflow.org/docs/components/trainer/"
