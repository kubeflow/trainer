def train_model():
    # 1. IMPORTS
    # ---
    import json
    import os  # To navigate file paths
    import random
    from datetime import datetime

    import torch
    import torch.distributed as dist
    import torch.nn as nn
    import torchaudio
    from torch.nn.parallel import DistributedDataParallel as DDP
    from torch.utils.data import DataLoader, Dataset
    from torch.utils.data.distributed import DistributedSampler
    from torch.utils.tensorboard import SummaryWriter

    debug = True

    # Its job: Load an audio file, convert it to a spectrogram,
    # and return it with its numerical label.
    def load_data(data_path):
        audio_info = []
        label_map = {}
        label_map_reverse = {}
        # Walk through the data directory to find all audio files
        # full_data_path = os.path.join(os.path.dirname(__file__), data_path)
        full_data_path = data_path

        # Get all subdirectories (word labels), excluding special directories
        labels = []
        for item in os.listdir(full_data_path):
            item_path = os.path.join(full_data_path, item)
            if (
                os.path.isdir(item_path)
                and not item.startswith("_")
                and item != "LICENSE"
            ):
                labels.append(item)

        # Sort labels for consistent mapping
        labels.sort()

        # Create label to integer mapping
        label_map = {label: idx for idx, label in enumerate(labels)}
        label_map_reverse = {idx: label for label, idx in enumerate(labels)}

        # Collect all audio files
        for label in labels:
            label_dir = os.path.join(full_data_path, label)
            for filename in os.listdir(label_dir):
                if filename.endswith(".wav"):
                    # Store relative path from speech-recognition folder
                    relative_path = os.path.join(data_path, label, filename)
                    audio_info.append({"filename": relative_path, "label": label})
        return audio_info, label_map, label_map_reverse

    def split_data(audio_info):
        # Don't shuffle here - let DistributedSampler handle shuffling
        # This ensures proper distributed sampling
        random.seed(41)
        random.shuffle(audio_info)
        print(f"audio info length: {len(audio_info)}")
        train_size = int(len(audio_info) * 0.95)
        val_size = int(len(audio_info) * 0.03)
        test_size = len(audio_info) - train_size - val_size
        print(f"train size: {train_size}, val size: {val_size}, test size: {test_size}")

        audio_info_training = audio_info[:train_size]
        audio_info_validation = audio_info[train_size: train_size + val_size]
        audio_info_test = audio_info[train_size + val_size:]
        return audio_info_training, audio_info_validation, audio_info_test

    class SpeechCommandsDataset(Dataset):
        def __init__(
            self, audio_info, label_map, label_map_reverse, data_path_prefix=None
        ):
            self.data_path_prefix = data_path_prefix
            self.audio_info = audio_info
            self.label_map = label_map
            self.label_map_reverse = label_map_reverse
            self.transform = torchaudio.transforms.MelSpectrogram(n_mels=128)

            print(json.dumps(self.audio_info[0:3], indent=4))
            print(self.label_map)

        def __len__(self):
            return len(self.audio_info)

        def __getitem__(self, index):
            audio_info = self.audio_info[index]

            # Build full path to audio file
            if self.data_path_prefix is None:
                audio_path = os.path.join(
                    os.path.dirname(__file__), audio_info["filename"]
                )
            else:
                audio_path = os.path.join(self.data_path_prefix, audio_info["filename"])

            # Load the audio file
            waveform, sample_rate = torchaudio.load(audio_path)

            # Transform to spectrogram
            spectrogram = self.transform(waveform)
            # print(f"Spectrogram shape: {spectrogram.shape}")

            # Get the numerical label from the word label
            label = self.label_map[audio_info["label"]]

            return spectrogram, label

    def collate_fn_spectrogram(batch):
        # batch is a list of tuples (spectrogram, label)

        # Let's set our target length
        target_length = 81

        spectrograms = []
        labels = []

        # Loop through each item in the batch
        for spec, label in batch:
            # spec shape is (1, num_features, time)
            current_length = spec.shape[2]

            # --- Padding or Truncating ---
            if current_length < target_length:
                # Pad with zeros if it's too short
                padding_needed = target_length - current_length
                # torch.pad takes (data, (pad_left, pad_right, pad_top, pad_bottom, ...))
                spec = torch.nn.functional.pad(spec, (0, padding_needed))
            elif current_length > target_length:
                # Truncate if it's too long
                spec = spec[:, :, :target_length]

            spectrograms.append(spec)
            labels.append(label)

        # Stack them into a single batch tensor
        spectrograms_batch = torch.cat(spectrograms, dim=0)
        labels_batch = torch.tensor(labels)

        return spectrograms_batch, labels_batch

    # Its job: Define the Transformer architecture.
    class AudioTransformer(nn.Module):
        def __init__(self, num_input_features=128, num_classes=35, dropout=0.1):
            super().__init__()
            # Using PyTorch's pre-built Transformer components
            encoder_layer = nn.TransformerEncoderLayer(
                d_model=num_input_features, nhead=4, batch_first=True, dropout=dropout
            )
            self.transformer_encoder = nn.TransformerEncoder(
                encoder_layer, num_layers=4
            )
            self.output_layer = nn.Linear(num_input_features, num_classes)

        def forward(self, spectrogram_batch):
            # Input shape needs to be (batch, time, features) for batch_first=True
            # Spectrograms are often (batch, features, time), so we might need to permute
            x = spectrogram_batch.permute(0, 2, 1)

            x = self.transformer_encoder(x)
            x = x.mean(dim=1)  # Average over the time dimension
            predictions = self.output_layer(x)
            return predictions

    class Trainer:
        def __init__(
            self,
            model,
            train_loader,
            val_loader,
            test_loader,
            optimizer,
            loss_fn,
            scheduler,
            device,
            total_epochs,
            exp_path=None,
            rank=None,
        ):
            self.model = model
            self.train_loader = train_loader
            self.val_loader = val_loader
            self.test_loader = test_loader
            self.optimizer = optimizer
            self.loss_fn = loss_fn
            self.scheduler = scheduler
            self.device = device
            self.total_epochs = total_epochs
            self.best_val_accuracy = 0.0
            self.step = 0
            self.timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
            self.total_steps = len(self.train_loader) * self.total_epochs
            if exp_path is None:
                self.exp_path = f"/data/speech-recognition/runs/exp-{self.timestamp}"
            else:
                self.exp_path = exp_path
            print(f"Experiment path: {self.exp_path}")

            self.rank = rank
            self.is_main_process = self.rank == 0

            if self.is_main_process:
                self.writer = SummaryWriter(f"{self.exp_path}/logs")
            else:
                self.writer = None

        def _train_one_epoch(self, epoch):
            self.train_loader.sampler.set_epoch(epoch)
            self.model.train()  # Ensure model is in training mode
            epoch_loss = 0.0
            num_batches = 0

            for batch_idx, (spectrograms, labels) in enumerate(self.train_loader):
                spectrograms = spectrograms.to(self.device)
                labels = labels.to(self.device)

                # 1. PREDICT: Pass data through the model
                predictions = self.model(spectrograms)

                # 2. COMPARE: Calculate the error
                loss = self.loss_fn(predictions, labels)

                # 3. ADJUST: Update the model's weights
                self.optimizer.zero_grad()
                loss.backward()
                # Add gradient clipping to prevent explosion
                torch.nn.utils.clip_grad_norm_(self.model.parameters(), max_norm=1.0)
                self.optimizer.step()

                # Update tracking variables
                epoch_loss += loss.item()
                num_batches += 1
                self.step += 1

                # Log to TensorBoard every step
                if self.is_main_process:
                    self.writer.add_scalar("Loss/Train", loss.item(), self.step)
                    # Print progress every 10 steps or at the end of each epoch
                    if (batch_idx + 1) % 10 == 0 or (batch_idx + 1) == len(
                        self.train_loader
                    ):
                        avg_loss = epoch_loss / num_batches
                        print(
                            f"Epoch {epoch+1:2d}/{self.total_epochs} "
                            f"| Step {self.step:4d}/{self.total_steps} | "
                            f"Batch {batch_idx+1:3d}/{len(self.train_loader)} | "
                            f"Loss: {loss.item():.6f} | Avg Loss: {avg_loss:.6f}"
                        )

            # Log epoch average loss to TensorBoard
            avg_epoch_loss = epoch_loss / num_batches
            if self.is_main_process:
                self.writer.add_scalar("Loss/Epoch_Avg", avg_epoch_loss, epoch + 1)
                # Print epoch summary
                print(
                    f"Epoch {epoch+1:2d}/{self.total_epochs} completed "
                    f"| Avg Loss: {avg_epoch_loss:.6f}"
                )
                print("-" * 80)

        def _validate_one_epoch(self, epoch, loader=None):
            # Single-machine validation (only runs on rank 0)
            self.model.eval()  # 1. Switch to evaluation mode
            val_loss = 0
            correct = 0
            total = 0

            print("  Starting validation...")

            with torch.no_grad():  # 2. Do not compute gradients within this code block
                for batch_idx, (spectrograms, labels) in enumerate(loader):
                    spectrograms = spectrograms.to(self.device)
                    labels = labels.to(self.device)

                    # Only perform prediction and calculate loss
                    predictions = self.model(spectrograms)
                    loss = self.loss_fn(predictions, labels)

                    val_loss += loss.item()

                    # Calculate accuracy
                    _, predicted_labels = torch.max(predictions.data, 1)
                    total += labels.size(0)
                    correct += (predicted_labels == labels).sum().item()

                    # Print validation progress every 10 batches
                    if (batch_idx + 1) % 10 == 0 or (batch_idx + 1) == len(loader):
                        current_accuracy = 100 * correct / total
                        print(
                            f"    Validation Batch {batch_idx+1:3d}/{len(loader)} | "
                            f"Current Accuracy: {current_accuracy:.2f}%"
                        )

            # Simple single-machine calculation
            avg_loss = val_loss / len(loader)
            val_accuracy = 100 * correct / total

            print(
                f"  Validation completed | Loss: {avg_loss:.6f} | Accuracy: {val_accuracy:.2f}%"
            )

            if val_accuracy > self.best_val_accuracy:
                self.best_val_accuracy = val_accuracy
                print(
                    f"New best validation accuracy: {self.best_val_accuracy:.2f}%. Saving model..."
                )
                model_folder = f"{self.exp_path}/models"
                os.makedirs(model_folder, exist_ok=True)
                # Handle both DDP and non-DDP models
                if hasattr(self.model, "module"):
                    model_state = self.model.module.state_dict()
                else:
                    model_state = self.model.state_dict()
                torch.save(model_state, f"{model_folder}/best-epoch{epoch}.pth")

                if self.writer:
                    self.writer.add_scalar("Loss/Val", val_loss, epoch + 1)
                    self.writer.add_scalar("Accuracy/Val", val_accuracy, epoch + 1)

        def train(self):
            print("Starting training...")
            for epoch in range(self.total_epochs):
                self._train_one_epoch(epoch)
                # Only validate if val_loader is available (single-machine validation)
                if self.val_loader is not None:
                    self._validate_one_epoch(epoch, self.val_loader)

                # Synchronize all processes after validation
                if dist.is_initialized():
                    dist.barrier()  # Wait for rank 0 to finish validation

                if self.scheduler:
                    self.scheduler.step()
                if self.is_main_process:
                    print("-" * 80)

            if self.is_main_process:
                self.writer.close()
                print("Training complete!")

        def test(self):
            # Only run test on rank 0 (single-machine testing)
            if self.test_loader is None:
                return

            print("Starting test...")
            self.model.eval()  # 1. Switch to evaluation mode
            val_loss = 0
            correct = 0
            total = 0

            with torch.no_grad():  # 2. Do not compute gradients within this code block
                for batch_idx, (spectrograms, labels) in enumerate(self.test_loader):
                    spectrograms = spectrograms.to(self.device)
                    labels = labels.to(self.device)

                    # Only perform prediction and calculate loss
                    predictions = self.model(spectrograms)
                    loss = self.loss_fn(predictions, labels)

                    val_loss += loss.item()

                    # Calculate accuracy
                    _, predicted_labels = torch.max(predictions.data, 1)
                    total += labels.size(0)
                    correct += (predicted_labels == labels).sum().item()

                    # Print test progress every 10 batches
                    if (batch_idx + 1) % 10 == 0 or (batch_idx + 1) == len(
                        self.test_loader
                    ):
                        current_accuracy = 100 * correct / total
                        print(
                            f"    Test Batch {batch_idx+1:3d}/{len(self.test_loader)} | "
                            f"Current Accuracy: {current_accuracy:.2f}%"
                        )

            # Simple single-machine calculation
            avg_loss = val_loss / len(self.test_loader)
            val_accuracy = 100 * correct / total

            print(
                f"  Test completed | Loss: {avg_loss:.6f} | Accuracy: {val_accuracy:.2f}%"
            )
            print("Test complete!")

    def setup_ddp():
        """Initialize DDP process group"""
        # This line is key: bind unique GPU for current process
        if (
            "LOCAL_RANK" not in os.environ
            or "RANK" not in os.environ
            or "WORLD_SIZE" not in os.environ
        ):
            print("LOCAL_RANK, RANK, and WORLD_SIZE is not set, will skip using DDP")
            return torch.device("cuda") if torch.cuda.is_available() else "cpu", 0, 0
        print(
            f"LOCAL_RANK: {os.environ['LOCAL_RANK']}, RANK: {os.environ['RANK']}, "
            f"WORLD_SIZE: {os.environ['WORLD_SIZE']}"
        )

        local_rank = int(os.environ["LOCAL_RANK"])

        if torch.cuda.is_available():
            dist.init_process_group(backend="nccl")
            device = torch.device("cuda", local_rank)
            torch.cuda.set_device(device)
            print(f"Using device: {device}")

        else:
            device = torch.device("cpu")
            dist.init_process_group(backend="gloo")

        rank = torch.distributed.get_rank()
        print(
            f"Rank(dist.get_rank()): {rank}, "
            f"Rank(os.environ['RANK']): {os.environ['RANK']}, "
            f"Local Rank(os.environ['LOCAL_RANK']): {local_rank}"
        )

        return device, local_rank, rank

    def cleanup_ddp():
        """Destroy process group"""
        dist.destroy_process_group()

    def train():
        # This line will automatically select GPU (if available), otherwise fall back to CPU
        device, local_rank, rank = setup_ddp()

        print(f"Using device: {device}")

        # Instantiate the Dataset and DataLoader
        print("start loading dataset")
        data_path_prefix = "/data/SpeechCommands/speech_commands_v0.02"
        audio_info, label_map, label_map_reverse = load_data(data_path_prefix)

        audio_info_training, audio_info_validation, audio_info_test = split_data(
            audio_info
        )

        if debug:
            # Use more data for debug: 4000 train, 500 val, 500 test
            audio_info_training = audio_info_training[:4000]
            audio_info_validation = audio_info_validation[:500]
            audio_info_test = audio_info_test[:500]
            print(
                f"Debug mode: using train={len(audio_info_training)}, "
                f"val={len(audio_info_validation)}, "
                f"test={len(audio_info_test)}"
            )

        train_dataset = SpeechCommandsDataset(
            audio_info_training, label_map, label_map_reverse, data_path_prefix
        )

        val_dataset = SpeechCommandsDataset(
            audio_info_validation, label_map, label_map_reverse, data_path_prefix
        )
        test_dataset = SpeechCommandsDataset(
            audio_info_test, label_map, label_map_reverse, data_path_prefix
        )

        print("dataset loaded")

        print("start init data loader")

        train_sampler = DistributedSampler(train_dataset, shuffle=True)
        # Adjust batch size for debug mode
        batch_size = 64 if debug else 256
        train_loader = DataLoader(
            train_dataset,
            batch_size=batch_size,
            shuffle=False,
            sampler=train_sampler,
            collate_fn=collate_fn_spectrogram,
        )
        # Use single-machine validation (only rank 0)
        if rank == 0:
            val_loader = DataLoader(
                val_dataset,
                batch_size=batch_size,
                shuffle=False,
                collate_fn=collate_fn_spectrogram,
            )
            test_loader = DataLoader(
                test_dataset,
                batch_size=batch_size,
                shuffle=False,
                collate_fn=collate_fn_spectrogram,
            )
        else:
            val_loader = None
            test_loader = None

        print("data loader initialized")

        # Instantiate the Model, Loss Function, and Optimizer
        model = AudioTransformer().to(device)

        # Create DDP model - different parameters for CPU vs GPU
        if torch.cuda.is_available() and device.type == "cuda":
            ddp_model = DDP(model, device_ids=[local_rank])
        else:
            # For CPU training, don't specify device_ids
            ddp_model = DDP(model)

        loss_fn = nn.CrossEntropyLoss()
        world_size = dist.get_world_size() if dist.is_initialized() else 1
        # Use linear scaling with a more conservative approach
        base_lr = 0.001
        lr = (
            base_lr * min(world_size, 2) if not debug else base_lr
        )  # No scaling in debug
        print(f"Using learning rate: {lr} (world_size: {world_size}, debug: {debug})")
        optimizer = torch.optim.Adam(ddp_model.parameters(), lr=lr)
        scheduler = torch.optim.lr_scheduler.StepLR(optimizer, step_size=3, gamma=0.9)

        # Generate timestamp for this experiment
        timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
        print(f"Experiment timestamp: {timestamp}")

        # --- 2. Create and start Trainer ---
        total_epochs = 10 if debug else 15
        trainer = Trainer(
            ddp_model,
            train_loader,
            val_loader,
            test_loader,
            optimizer,
            loss_fn,
            scheduler,
            device,
            total_epochs,
            rank=rank,
        )
        trainer.train()

        # --- 3. (Optional) Final test ---
        trainer.test()  # You can add a .test() method to Trainer

        # Synchronize all processes after test
        if dist.is_initialized():
            dist.barrier()  # Wait for rank 0 to finish test

        cleanup_ddp()

        if rank == 0:
            print("Training complete!")
            print("To view TensorBoard, run: tensorboard --logdir=runs")

    train()


if __name__ == "__main__":
    train_model()
