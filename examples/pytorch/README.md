# PyTorch Examples

This directory contains examples for training PyTorch models using the Kubeflow Trainer SDK. These notebooks demonstrate how to transition from local development to distributed training on Kubernetes using the Trainer V2 SDK.

## Examples

| Task | Model | Dataset | Notebook |
| :--- | :--- | :--- | :--- |
| Image Classification | CNN | Fashion MNIST | [mnist.ipynb](./image-classification/mnist.ipynb) |
| Question Answering | DistilBERT | SQuAD | [fine-tune-distilbert.ipynb](./question-answering/fine-tune-distilbert.ipynb) |
| Speech Recognition | Transformer | Speech Commands | [speech-recognition.ipynb](./speech-recognition/speech-recognition.ipynb) |

## Updated Workflows

- **Image Classification (mnist.ipynb):** Refactored to demonstrate the official V2 DDP training workflow on the Fashion MNIST dataset.
- **Question Answering (fine-tune-distilbert.ipynb):** Updated to demonstrate fine-tuning with Hugging Face integration, including critical fixes for offset mapping, Fast Tokenizers, and Accelerate backend requirements.
- **Speech Recognition (speech-recognition.ipynb):** Added a new transformer-based workflow for speech classification. Implements custom audio preprocessing/sampling using `torchaudio` and `soundfile` with native DDP support.

## Key Improvements in V2 SDK Examples

- **SDK V2 Native:** Migrated all training logic to use `TrainerClient` and `CustomTrainer` for a more consistent API experience.
- **Cross-Platform Compatibility:** Standardized the distributed backend to `gloo` where necessary to ensure notebooks run successfully on Windows, macOS, and Linux local environments.
- **Robust Verification Logic:** Implemented a `KUBEFLOW_TRAINER_TEST` environment flag. For complex tasks (LLM/Audio), this allows for instant logic verification using `max_steps=1` and tiny data subsets without requiring high-compute resources.
- **Unified Execution Models:** Each notebook provides tested paths for:
  - **Direct Python:** Quick kernel-level experimentation.
  - **SDK Local:** Isolated environment verification using `LocalProcessBackendConfig`.
  - **Cluster Scaling:** Distributed execution on Kubernetes with `num_nodes` scaling.
- **Environment Stability:** Added explicit dependency checks and installation steps to ensure repeatable runs across different notebook environments.

## How to use

1. **Install the Kubeflow SDK**:
   ```bash
   pip install kubeflow-trainer
   ```

2. **Run the Notebooks**:
   Open the notebooks in your favorite editor (Jupyter, VS Code, etc.) and follow the instructions to run locally or on a Kubernetes cluster.

## Resources

- [Kubeflow Trainer Documentation](https://www.kubeflow.org/docs/components/trainer/)
- [PyTorch Distributed Overview](https://pytorch.org/docs/stable/distributed.html)
