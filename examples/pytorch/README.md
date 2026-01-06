# PyTorch Examples

This directory contains examples for training PyTorch models using the Kubeflow Trainer SDK.

## Examples

| Task | Model | Dataset | Notebook |
| :--- | :--- | :--- | :--- |
| Image Classification | CNN | Fashion MNIST | [mnist.ipynb](./image-classification/mnist.ipynb) |
| Question Answering | DistilBERT | SQuAD | [fine-tune-distilbert.ipynb](./question-answering/fine-tune-distilbert.ipynb) |

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
