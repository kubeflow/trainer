# TensorFlow Object Detection Example

This example demonstrates how to train a YOLO (You Only Look Once) object detection model using TensorFlow and Kubeflow Trainer for distributed training.

## Overview

This notebook shows you how to:
- Build a YOLOv3-tiny model using TensorFlow/Keras
- Train on the COCO dataset (or a subset for demonstration)
- Scale training across multiple nodes using Kubeflow TrainJob
- Use TensorFlow's distributed training strategies

## Prerequisites

### Local Development
- Python 3.11+
- TensorFlow 2.15+
- Kubeflow SDK: `pip install -U kubeflow`

### Cluster Requirements
- Kubernetes cluster with Kubeflow Trainer installed
- GPU nodes recommended (but CPU training is supported)
- Sufficient storage for dataset (COCO dataset is ~20GB)

## Quick Start

### 1. Install Dependencies

```bash
pip install -U kubeflow tensorflow tensorflow-datasets opencv-python-headless
```

### 2. Run the Notebook

Open and run `yolo-object-detection.ipynb` to:
- Train locally with TensorFlow
- Scale to distributed training with Kubeflow TrainJob

## Dataset

This example uses a subset of the [COCO dataset](https://cocodataset.org/) for demonstration purposes. The full COCO dataset contains:
- 118K training images
- 5K validation images
- 80 object categories

For faster experimentation, the notebook uses a smaller subset by default.

## Model Architecture

The example implements YOLOv3-tiny, a lightweight version of YOLO that:
- Uses fewer convolutional layers than full YOLOv3
- Achieves faster inference with reasonable accuracy
- Is suitable for edge devices and resource-constrained environments

## Training Configuration

Default hyperparameters:
- **Batch size**: 16 per device
- **Learning rate**: 0.001
- **Epochs**: 10
- **Image size**: 416x416
- **Optimizer**: Adam

## Distributed Training

The example demonstrates TensorFlow's `MultiWorkerMirroredStrategy` for distributed training:
- Synchronous data parallelism across multiple workers
- Automatic gradient aggregation
- Efficient communication using NCCL (GPU) or Gloo (CPU)

## Expected Results

After training, you should see:
- Training loss decreasing over epochs
- Object detection predictions on sample images
- Model checkpoints saved for inference

## Customization

You can customize the training by modifying:
- **Dataset**: Use your own dataset or different COCO subsets
- **Model**: Adjust YOLOv3-tiny architecture or use YOLOv3/YOLOv4
- **Hyperparameters**: Tune batch size, learning rate, epochs
- **Resources**: Scale to more nodes or GPUs

## Troubleshooting

### Out of Memory (OOM)
- Reduce batch size
- Use smaller image size (e.g., 320x320)
- Enable mixed precision training

### Slow Training
- Use GPU nodes instead of CPU
- Increase number of workers
- Enable XLA compilation

### Dataset Download Issues
- Check internet connectivity
- Verify sufficient disk space
- Use cached dataset if available

## References

- [YOLO: Real-Time Object Detection](https://pjreddie.com/darknet/yolo/)
- [TensorFlow Object Detection API](https://github.com/tensorflow/models/tree/master/research/object_detection)
- [COCO Dataset](https://cocodataset.org/)
- [Kubeflow Trainer Documentation](https://www.kubeflow.org/docs/components/trainer/)

## Contributing

Found an issue or want to improve this example? Please [open an issue](https://github.com/kubeflow/trainer/issues/new) or submit a pull request!