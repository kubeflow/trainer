# PyTorch Examples

This directory contains examples for training PyTorch models using the Kubeflow Trainer SDK.

### Examples

| Use Case | Script | Notebook |
| :--- | :--- | :--- |
| **Image Classification** | [train_mnist.py](image-classification/train_mnist.py) | [mnist.ipynb](image-classification/mnist.ipynb) |

### Prerequisites

To run these examples, install the Kubeflow SDK:
```bash
pip install -U kubeflow
```

### How to Run

These standalone scripts are designed for automated workflows and production training. They automatically handle distributed setup and dependency installation on the cluster.

**Submit an MNIST training job:**
```bash
python image-classification/train_mnist.py --nodes 1
```

**Verify locally (no Kubernetes needed):**
You can verify the training logic on your local machine using the `--test` flag:
```bash
python image-classification/train_mnist.py --test
```

For interactive experimentation, you can also use the corresponding Jupyter notebooks in each subdirectory.
