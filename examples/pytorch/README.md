# PyTorch Examples

This directory contains examples for training PyTorch models using the Kubeflow Trainer SDK.

### Examples

| Task | Model | Dataset | Notebook |
| :--- | :--- | :--- | :--- |
| Image Classification | CNN | Fashion MNIST | [mnist.ipynb](./image-classification/mnist.ipynb) |
| Question Answering | DistilBERT | SQuAD | [fine-tune-distilbert.ipynb](./question-answering/fine-tune-distilbert.ipynb) |
| Speech Recognition | Transformer | Speech Commands | [speech-recognition.ipynb](./speech-recognition/speech-recognition.ipynb) |

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
