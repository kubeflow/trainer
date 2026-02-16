# PyTorch Examples

This directory contains examples for training PyTorch models using the Kubeflow Trainer SDK.

## Examples

| Task | Model | Dataset | Notebook |
| :--- | :--- | :--- | :--- |
| Image Classification | CNN | Fashion MNIST | [mnist.ipynb](./image-classification/mnist.ipynb) |
| Question Answering | DistilBERT | SQuAD | [fine-tune-distilbert.ipynb](./question-answering/fine-tune-distilbert.ipynb) |
| Speech Recognition | Transformer | Speech Commands | [speech-recognition.ipynb](./speech-recognition/speech-recognition.ipynb) |
| Audio Classification | CNN (M5) | GTZAN | [audio-classification.ipynb](./audio-classification/audio-classification.ipynb) |
| Data Caching | [Model] | [Dataset] | [data-cache-example.ipynb](./data-cache/data-cache-example.ipynb) |

## Prerequisites

To run these examples, install the Kubeflow SDK:
```bash
pip install -U kubeflow
```
