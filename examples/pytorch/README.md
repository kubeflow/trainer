# PyTorch Examples

This directory contains examples for training PyTorch models using the Kubeflow Trainer SDK.

## Examples

| Task | Model | Dataset | Notebook |
| :--- | :--- | :--- | :--- |
| Image Classification | CNN | Fashion MNIST | [mnist.ipynb](./image-classification/mnist.ipynb) |
| Question Answering | DistilBERT | SQuAD | [fine-tune-distilbert.ipynb](./question-answering/fine-tune-distilbert.ipynb) |
| Speech Recognition | Transformer | Speech Commands | [speech-recognition.ipynb](./speech-recognition/speech-recognition.ipynb) |
| Audio Classification | CNN (M5) | GTZAN | [audio-classification.ipynb](./audio-classification/audio-classification.ipynb) |
| Fine-Tune LLM | BERT | Amazon Reviews | [fine-tune-llm-with-data-cache.ipynb](./fine-tune-llm-with-data-cache/fine-tune-llm-with-data-cache.ipynb) |

## Prerequisites

To run these examples, install the Kubeflow SDK:
```bash
pip install -U kubeflow
```

## Workflows

- **Image Classification (mnist.ipynb):** Demonstrates distributed training on the Fashion MNIST dataset using CNNs.
- **Question Answering (fine-tune-distilbert.ipynb):** Fine-tuning DistilBERT on the SQuAD dataset with Hugging Face integration.
- **Speech Recognition (speech-recognition.ipynb):** Spoken word classification using an Audio Transformer on the Speech Commands dataset.
- **Audio Classification (audio-classification.ipynb):** Music genre classification using the M5 1D CNN architecture on the GTZAN dataset.
- **Fine-Tune LLM with Data Cache:** Demonstrates fine-tuning BERT with efficient data streaming from a distributed cache using Iceberg tables.