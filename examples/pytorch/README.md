# PyTorch Examples

This directory contains examples for training PyTorch models using the Kubeflow Trainer SDK.

### Examples

| Task | Model | Dataset | Notebook |
| :--- | :--- | :--- | :--- |
| Image Classification | CNN | Fashion MNIST | [mnist.ipynb](./image-classification/mnist.ipynb) |
| Question Answering | DistilBERT | SQuAD | [fine-tune-distilbert.ipynb](./question-answering/fine-tune-distilbert.ipynb) |
| Speech Recognition | Transformer | Speech Commands | [speech-recognition.ipynb](./speech-recognition/speech-recognition.ipynb) |
| Audio Classification | CNN (M5) | GTZAN | [audio-classification.ipynb](./audio-classification/audio-classification.ipynb) |

### Prerequisites

To run these examples, install the Kubeflow SDK:
```bash
pip install -U kubeflow
```

## Updated Workflows

- **Image Classification (mnist.ipynb):** Refactored to demonstrate the official V2 DDP training workflow on the Fashion MNIST dataset.
- **Question Answering (fine-tune-distilbert.ipynb):** Updated to demonstrate fine-tuning with Hugging Face integration, including critical fixes for offset mapping, Fast Tokenizers, and Accelerate backend requirements.
- **Speech Recognition (speech-recognition.ipynb):** Implements spoken word classification using an Audio Transformer on the Speech Commands dataset.
- **Audio Classification (audio-classification.ipynb):** Demonstrates general audio classification using the GTZAN music genre dataset. Uses the M5 1D CNN architecture for processing raw audio waveforms.

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
