# Testing TensorFlow YOLO Example on OpenShift with NVIDIA GPUs

This guide helps you test the TensorFlow YOLO object detection example on your OpenShift cluster with NVIDIA RTX PRO 6000 Blackwell GPUs.

## Cluster Information

Your cluster has:
- **5 worker nodes** with RHEL CoreOS 9.6
- **NVIDIA RTX PRO 6000 Blackwell GPUs**: 2 GPUs per node (97GB memory each)
- **CUDA Driver**: Installed and configured
- **GPU Operator**: Deployed and running
- **Container Runtime**: CRI-O 1.33.12

## Prerequisites

### 1. Install Kubeflow Trainer

Since Kubeflow Trainer is not yet installed, you need to install it first:

```bash
# Create namespace
oc create namespace kubeflow

# Install Kubeflow Trainer using kustomize
oc apply --server-side -k "https://github.com/kubeflow/trainer.git/manifests/base?ref=master"

# Wait for deployment
oc wait --for=condition=available --timeout=300s deployment/trainer-controller-manager -n kubeflow

# Verify installation
oc get clustertrainingruntimes
```

Expected output:
```
NAME                 AGE
deepspeed            1m
jax-distributed      1m
mlx-distributed      1m
mpi-distributed      1m
torch-distributed    1m
xgboost-distributed  1m
```

### 2. Install Python Dependencies Locally

```bash
cd examples/tensorflow/object-detection
pip install -r requirements.txt
```

## Testing Options

### Option 1: Quick Test with Standalone Pod (No Kubeflow Trainer)

Test TensorFlow GPU access first:

```bash
# Create test namespace
oc create namespace tf-test

# Create a simple TensorFlow GPU test pod
cat <<EOF | oc apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: tensorflow-gpu-test
  namespace: tf-test
spec:
  restartPolicy: Never
  containers:
  - name: tensorflow
    image: tensorflow/tensorflow:2.15.0-gpu
    command: ["python", "-c"]
    args:
    - |
      import tensorflow as tf
      print(f"TensorFlow version: {tf.__version__}")
      print(f"GPU devices: {tf.config.list_physical_devices('GPU')}")
      print(f"Num GPUs: {len(tf.config.list_physical_devices('GPU'))}")
      
      # Test GPU computation
      with tf.device('/GPU:0'):
        a = tf.constant([[1.0, 2.0], [3.0, 4.0]])
        b = tf.constant([[1.0, 1.0], [0.0, 1.0]])
        c = tf.matmul(a, b)
        print(f"Matrix multiplication result: {c}")
      print("GPU test successful!")
    resources:
      limits:
        nvidia.com/gpu: 1
      requests:
        nvidia.com/gpu: 1
EOF

# Check logs
oc logs -f tensorflow-gpu-test -n tf-test

# Cleanup
oc delete pod tensorflow-gpu-test -n tf-test
```

### Option 2: Test with Kubeflow Trainer (Recommended)

After installing Kubeflow Trainer, create a test TrainJob:

```bash
# Create namespace for training
oc create namespace ml-training

# Create a simple TensorFlow TrainJob
cat <<EOF | oc apply -f -
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: tensorflow-yolo-test
  namespace: ml-training
spec:
  runtimeRef:
    name: torch-distributed  # Using torch-distributed runtime
  trainer:
    image: tensorflow/tensorflow:2.15.0-gpu
    command:
      - python
      - -c
      - |
        import os
        import json
        import tensorflow as tf
        
        print(f"TensorFlow version: {tf.__version__}")
        print(f"GPU devices: {tf.config.list_physical_devices('GPU')}")
        
        # Check for distributed training config
        tf_config = os.environ.get('TF_CONFIG')
        if tf_config:
            print(f"TF_CONFIG: {tf_config}")
            config = json.loads(tf_config)
            print(f"Task type: {config['task']['type']}")
            print(f"Task index: {config['task']['index']}")
        
        # Simple training test
        print("Running simple training test...")
        model = tf.keras.Sequential([
            tf.keras.layers.Dense(10, activation='relu', input_shape=(20,)),
            tf.keras.layers.Dense(1)
        ])
        model.compile(optimizer='adam', loss='mse')
        
        import numpy as np
        x = np.random.rand(100, 20)
        y = np.random.rand(100, 1)
        
        history = model.fit(x, y, epochs=3, verbose=1)
        print(f"Final loss: {history.history['loss'][-1]:.4f}")
        print("Training test successful!")
    numNodes: 2
    resourcesPerNode:
      limits:
        nvidia.com/gpu: 1
        memory: 16Gi
        cpu: 4
      requests:
        nvidia.com/gpu: 1
        memory: 16Gi
        cpu: 4
EOF

# Monitor the TrainJob
oc get trainjob tensorflow-yolo-test -n ml-training -w

# Check pods
oc get pods -n ml-training -l jobset.sigs.k8s.io/jobset-name=tensorflow-yolo-test

# View logs from worker 0
oc logs -f -n ml-training -l jobset.sigs.k8s.io/jobset-name=tensorflow-yolo-test,jobset.sigs.k8s.io/replicatedjob-replicas=0

# Cleanup
oc delete trainjob tensorflow-yolo-test -n ml-training
```

### Option 3: Test Full YOLO Example with Kubeflow SDK

Create a Python script to test the full example:

```python
# test_yolo_example.py
from kubeflow.trainer import CustomTrainer, TrainerClient

# Import the training function from the notebook
import sys
sys.path.append('.')

# Define a simplified test version
def test_yolo_training():
    import tensorflow as tf
    print(f"TensorFlow version: {tf.__version__}")
    print(f"GPUs available: {len(tf.config.list_physical_devices('GPU'))}")
    
    # Quick 2-epoch test
    from yolo_object_detection import train_yolo_object_detection
    train_yolo_object_detection(
        num_epochs=2,
        batch_size=8,
        learning_rate=0.001,
        image_size=416
    )

# Initialize Kubeflow client
client = TrainerClient()

# Submit training job
job_name = client.train(
    trainer=CustomTrainer(
        func=test_yolo_training,
        packages_to_install=["tensorflow==2.15.0", "numpy"],
        num_nodes=2,
        resources_per_node={
            "cpu": 4,
            "memory": "16Gi",
            "nvidia.com/gpu": 1,
        },
    ),
    runtime="torch-distributed",
)

print(f"TrainJob submitted: {job_name}")

# Wait and get logs
client.wait_for_job_status(job_name, status={"Running"})
for log in client.get_job_logs(job_name, follow=True):
    print(log, end='')
```

Run the test:
```bash
python test_yolo_example.py
```

## Verification Steps

### 1. Check GPU Allocation

```bash
# Check GPU resources on nodes
oc describe nodes | grep -A 5 "nvidia.com/gpu"

# Check GPU pods
oc get pods --all-namespaces -o json | jq '.items[] | select(.spec.containers[].resources.limits."nvidia.com/gpu" != null) | {name: .metadata.name, namespace: .metadata.namespace, gpus: .spec.containers[].resources.limits."nvidia.com/gpu"}'
```

### 2. Monitor Training Progress

```bash
# Watch TrainJob status
oc get trainjob -n ml-training -w

# Check pod events
oc get events -n ml-training --sort-by='.lastTimestamp'

# View detailed pod info
oc describe pod <pod-name> -n ml-training
```

### 3. Check GPU Utilization

```bash
# If you have access to the nodes, check nvidia-smi
oc debug node/<node-name>
chroot /host
nvidia-smi
```

## Troubleshooting

### Issue: Pods stuck in Pending

**Check:**
```bash
oc describe pod <pod-name> -n ml-training
```

**Common causes:**
- Insufficient GPU resources
- Node selector not matching
- Resource quotas exceeded

**Solution:**
```bash
# Check available GPUs
oc get nodes -o json | jq '.items[] | {name: .metadata.name, gpus: .status.allocatable."nvidia.com/gpu"}'

# Reduce num_nodes or GPUs per node in the TrainJob
```

### Issue: CUDA/GPU not detected

**Check:**
```bash
# Verify GPU operator is running
oc get pods -n nvidia-gpu-operator

# Check node labels
oc get nodes -L nvidia.com/gpu.present
```

**Solution:**
```bash
# Restart GPU operator if needed
oc rollout restart daemonset nvidia-driver-daemonset -n nvidia-gpu-operator
```

### Issue: Out of Memory (OOM)

**Solution:**
- Reduce batch_size in training function
- Reduce image_size (e.g., 320x320 instead of 416x416)
- Increase memory limits in TrainJob spec

### Issue: TensorFlow not using GPU

**Check in pod logs:**
```
Could not load dynamic library 'libcudart.so.11.0'
```

**Solution:**
- Use TensorFlow GPU image: `tensorflow/tensorflow:2.15.0-gpu`
- Verify CUDA compatibility with GPU driver

## Performance Optimization

### 1. Enable Mixed Precision Training

Add to training function:
```python
from tensorflow.keras import mixed_precision
mixed_precision.set_global_policy('mixed_float16')
```

### 2. Use XLA Compilation

```python
tf.config.optimizer.set_jit(True)
```

### 3. Optimize Data Pipeline

```python
dataset = dataset.prefetch(tf.data.AUTOTUNE)
dataset = dataset.cache()
```

## Expected Results

With 2 nodes × 1 GPU (NVIDIA RTX PRO 6000):
- **Training time**: ~5-10 minutes for 10 epochs (synthetic data)
- **GPU utilization**: 70-90%
- **Memory usage**: ~8-12GB per GPU
- **Throughput**: ~100-200 images/second

## Next Steps

1. ✅ Verify GPU access with standalone pod
2. ✅ Install Kubeflow Trainer
3. ✅ Test simple TensorFlow TrainJob
4. ✅ Run full YOLO example
5. ✅ Monitor and optimize performance
6. 📝 Document results and contribute back

## Support

If you encounter issues:
1. Check [Kubeflow Trainer documentation](https://www.kubeflow.org/docs/components/trainer/)
2. Review [TensorFlow GPU guide](https://www.tensorflow.org/install/gpu)
3. Ask in [#kubeflow-trainer Slack](https://kubeflow.slack.com)
4. Open an issue on [GitHub](https://github.com/kubeflow/trainer/issues)