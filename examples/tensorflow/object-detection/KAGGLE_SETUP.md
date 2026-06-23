# Using Kaggle COCO Dataset for YOLO Training

This guide explains how to use the COCO dataset from Kaggle for training the YOLO object detection model on your OpenShift cluster.

## Overview

The COCO (Common Objects in Context) dataset is available on Kaggle and contains:
- **118,287 training images**
- **5,000 validation images**
- **80 object categories**
- **Bounding box annotations**
- **Segmentation masks**

## Option 1: Using TensorFlow Datasets (Easiest)

The provided `yolo-coco-training.yaml` uses TensorFlow Datasets which automatically downloads COCO:

```bash
# Apply the TrainJob
oc apply -f yolo-coco-training.yaml

# Monitor progress
oc get trainjob yolo-coco-object-detection -w

# View logs
oc logs -f -l jobset.sigs.k8s.io/jobset-name=yolo-coco-object-detection
```

**Pros:**
- ✅ Automatic download and preprocessing
- ✅ No manual setup required
- ✅ Handles data format conversion

**Cons:**
- ⚠️ Downloads data every time (slower)
- ⚠️ Requires internet access from pods

## Option 2: Pre-download from Kaggle (Recommended for Production)

For production use, pre-download the dataset and mount it as a PersistentVolumeClaim (PVC).

### Step 1: Get Kaggle API Credentials

1. Go to https://www.kaggle.com/account
2. Click "Create New API Token"
3. Download `kaggle.json`
4. Create a Kubernetes secret:

```bash
# Create secret with Kaggle credentials
oc create secret generic kaggle-secret \
  --from-file=kaggle.json=/path/to/your/kaggle.json \
  -n default
```

### Step 2: Create PVC for Dataset

```bash
cat <<EOF | oc apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: coco-dataset-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 50Gi  # COCO dataset is ~25GB, allow extra space
  storageClassName: your-storage-class  # Update with your storage class
EOF
```

### Step 3: Download Dataset to PVC

Create a job to download the dataset:

```bash
cat <<EOF | oc apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: download-coco-dataset
  namespace: default
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: downloader
        image: python:3.11-slim
        command:
          - /bin/bash
          - -c
          - |
            set -e
            
            echo "Installing Kaggle CLI..."
            pip install -q kaggle
            
            echo "Setting up Kaggle credentials..."
            mkdir -p ~/.kaggle
            cp /kaggle-secret/kaggle.json ~/.kaggle/
            chmod 600 ~/.kaggle/kaggle.json
            
            echo "Downloading COCO 2017 dataset..."
            cd /data
            
            # Download COCO 2017 dataset
            kaggle datasets download -d awsaf49/coco-2017-dataset
            
            echo "Extracting dataset..."
            unzip -q coco-2017-dataset.zip
            
            echo "Organizing files..."
            # Organize into train/val structure
            mkdir -p coco2017/train coco2017/val
            mv train2017/* coco2017/train/ || true
            mv val2017/* coco2017/val/ || true
            mv annotations coco2017/
            
            echo "Cleaning up..."
            rm -f coco-2017-dataset.zip
            rm -rf train2017 val2017
            
            echo "✓ COCO dataset downloaded successfully!"
            ls -lh /data/coco2017/
        volumeMounts:
        - name: data
          mountPath: /data
        - name: kaggle-secret
          mountPath: /kaggle-secret
          readOnly: true
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: coco-dataset-pvc
      - name: kaggle-secret
        secret:
          secretName: kaggle-secret
EOF
```

Monitor the download:

```bash
# Watch job progress
oc get jobs download-coco-dataset -w

# View logs
oc logs -f job/download-coco-dataset
```

### Step 4: Use Pre-downloaded Dataset in TrainJob

Create a modified TrainJob that uses the PVC:

```yaml
apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainJob
metadata:
  name: yolo-coco-with-pvc
  namespace: default
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    image: tensorflow/tensorflow:2.15.0-gpu
    command:
      - python
      - -c
      - |
        import os
        import tensorflow as tf
        
        # Dataset is already downloaded at /data/coco2017
        COCO_DIR = '/data/coco2017'
        
        print(f"Using COCO dataset from: {COCO_DIR}")
        print(f"Train images: {len(os.listdir(os.path.join(COCO_DIR, 'train')))}")
        print(f"Val images: {len(os.listdir(os.path.join(COCO_DIR, 'val')))}")
        
        # Load images from directory
        train_dataset = tf.keras.preprocessing.image_dataset_from_directory(
            os.path.join(COCO_DIR, 'train'),
            image_size=(416, 416),
            batch_size=32,
            label_mode=None  # We'll load annotations separately
        )
        
        # Continue with training...
    
    numNodes: 2
    resourcesPerNode:
      limits:
        nvidia.com/gpu: 1
        memory: 32Gi
        cpu: 8
      requests:
        nvidia.com/gpu: 1
        memory: 32Gi
        cpu: 8
    
    # Mount the PVC with COCO dataset
    podTemplate:
      spec:
        volumes:
        - name: coco-data
          persistentVolumeClaim:
            claimName: coco-dataset-pvc
        containers:
        - name: trainer
          volumeMounts:
          - name: coco-data
            mountPath: /data
            readOnly: true
```

## Option 3: Using Kaggle Datasets Directly

You can also use Kaggle's API directly in your training code:

```python
# Install kaggle
import subprocess
subprocess.run(['pip', 'install', '-q', 'kaggle'])

# Set up credentials (from mounted secret)
import os
os.environ['KAGGLE_CONFIG_DIR'] = '/kaggle-secret'

# Download dataset
from kaggle.api.kaggle_api_extended import KaggleApi
api = KaggleApi()
api.authenticate()

# Download COCO dataset
api.dataset_download_files(
    'awsaf49/coco-2017-dataset',
    path='/tmp/coco',
    unzip=True
)
```

## Dataset Structure

After download, the COCO dataset structure:

```
coco2017/
├── train/
│   ├── 000000000009.jpg
│   ├── 000000000025.jpg
│   └── ... (118,287 images)
├── val/
│   ├── 000000000139.jpg
│   ├── 000000000285.jpg
│   └── ... (5,000 images)
└── annotations/
    ├── instances_train2017.json
    ├── instances_val2017.json
    ├── captions_train2017.json
    └── captions_val2017.json
```

## Annotation Format

COCO annotations are in JSON format:

```json
{
  "images": [
    {
      "id": 397133,
      "file_name": "000000397133.jpg",
      "width": 640,
      "height": 427
    }
  ],
  "annotations": [
    {
      "id": 1768,
      "image_id": 397133,
      "category_id": 44,  # bottle
      "bbox": [473.07, 395.93, 38.65, 28.67],  # [x, y, width, height]
      "area": 1109.0,
      "iscrowd": 0
    }
  ],
  "categories": [
    {"id": 1, "name": "person"},
    {"id": 2, "name": "bicycle"},
    ...
  ]
}
```

## Loading COCO Annotations

Example code to load and parse COCO annotations:

```python
import json
import tensorflow as tf

def load_coco_annotations(annotation_file):
    """Load COCO annotations from JSON file."""
    with open(annotation_file, 'r') as f:
        coco_data = json.load(f)
    
    # Create image_id to annotations mapping
    image_annotations = {}
    for ann in coco_data['annotations']:
        image_id = ann['image_id']
        if image_id not in image_annotations:
            image_annotations[image_id] = []
        image_annotations[image_id].append(ann)
    
    return coco_data, image_annotations

def convert_bbox_to_yolo(bbox, img_width, img_height):
    """Convert COCO bbox [x, y, w, h] to YOLO format [x_center, y_center, w, h] normalized."""
    x, y, w, h = bbox
    x_center = (x + w / 2) / img_width
    y_center = (y + h / 2) / img_height
    w_norm = w / img_width
    h_norm = h / img_height
    return [x_center, y_center, w_norm, h_norm]

# Load annotations
coco_data, image_annotations = load_coco_annotations('/data/coco2017/annotations/instances_train2017.json')

# Process each image
for img_info in coco_data['images']:
    image_id = img_info['id']
    annotations = image_annotations.get(image_id, [])
    
    # Convert bounding boxes to YOLO format
    yolo_labels = []
    for ann in annotations:
        bbox = ann['bbox']
        category_id = ann['category_id']
        yolo_bbox = convert_bbox_to_yolo(bbox, img_info['width'], img_info['height'])
        yolo_labels.append([category_id] + yolo_bbox)
```

## Performance Considerations

### Storage Requirements

| Component | Size | Notes |
|-----------|------|-------|
| Training images | ~18 GB | 118K images |
| Validation images | ~1 GB | 5K images |
| Annotations | ~500 MB | JSON files |
| **Total** | **~20 GB** | Compressed |
| **Extracted** | **~25 GB** | Uncompressed |

### Network Bandwidth

- **Download time**: 10-30 minutes (depends on connection)
- **Recommendation**: Pre-download to PVC for production

### Training Time Estimates

With 2 nodes × 1 GPU (NVIDIA RTX PRO 6000):

| Configuration | Time per Epoch | Total (100 epochs) |
|---------------|----------------|-------------------|
| Batch size 16 | ~45 minutes | ~75 hours |
| Batch size 32 | ~25 minutes | ~42 hours |
| Batch size 64 | ~15 minutes | ~25 hours |

With 4 nodes × 2 GPUs:

| Configuration | Time per Epoch | Total (100 epochs) |
|---------------|----------------|-------------------|
| Batch size 32 | ~8 minutes | ~13 hours |
| Batch size 64 | ~5 minutes | ~8 hours |

## Troubleshooting

### Issue: Kaggle API Authentication Failed

```bash
# Verify secret is created
oc get secret kaggle-secret

# Check secret content
oc get secret kaggle-secret -o yaml

# Recreate if needed
oc delete secret kaggle-secret
oc create secret generic kaggle-secret --from-file=kaggle.json=/path/to/kaggle.json
```

### Issue: Out of Disk Space

```bash
# Check PVC size
oc get pvc coco-dataset-pvc

# Increase PVC size
oc patch pvc coco-dataset-pvc -p '{"spec":{"resources":{"requests":{"storage":"100Gi"}}}}'
```

### Issue: Slow Download

- Use a node with good internet connectivity
- Consider downloading on a local machine and uploading to PVC
- Use a mirror or CDN if available

## Best Practices

1. **Use PVC for Production**: Pre-download dataset to avoid repeated downloads
2. **Enable Caching**: Use `dataset.cache()` to cache preprocessed data
3. **Parallel Loading**: Use `num_parallel_calls=tf.data.AUTOTUNE`
4. **Prefetching**: Always use `dataset.prefetch(tf.data.AUTOTUNE)`
5. **Data Augmentation**: Add augmentation to improve model generalization
6. **Validation Split**: Always validate on separate data
7. **Checkpointing**: Save checkpoints regularly during long training

## Next Steps

1. Download COCO dataset using one of the methods above
2. Run the training job: `oc apply -f yolo-coco-training.yaml`
3. Monitor training progress
4. Evaluate model performance
5. Fine-tune hyperparameters
6. Deploy trained model for inference

## Resources

- [COCO Dataset Website](https://cocodataset.org/)
- [Kaggle COCO Dataset](https://www.kaggle.com/datasets/awsaf49/coco-2017-dataset)
- [COCO API Documentation](https://github.com/cocodataset/cocoapi)
- [TensorFlow Datasets COCO](https://www.tensorflow.org/datasets/catalog/coco)