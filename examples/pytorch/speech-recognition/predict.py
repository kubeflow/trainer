import glob
import os
import random

import torch
import torch.nn as nn
import torchaudio


class AudioTransformer(nn.Module):
    def __init__(self, num_input_features=128, num_classes=35, dropout=0.1):
        super().__init__()
        # Using PyTorch's pre-built Transformer components
        encoder_layer = nn.TransformerEncoderLayer(
            d_model=num_input_features, nhead=4, batch_first=True, dropout=dropout
        )
        self.transformer_encoder = nn.TransformerEncoder(encoder_layer, num_layers=4)
        self.output_layer = nn.Linear(num_input_features, num_classes)

    def forward(self, spectrogram_batch):
        # Input shape needs to be (batch, time, features) for batch_first=True
        # Spectrograms are often (batch, features, time), so we might need to permute
        x = spectrogram_batch.permute(0, 2, 1)

        x = self.transformer_encoder(x)
        x = x.mean(dim=1)  # Average over the time dimension
        predictions = self.output_layer(x)
        return predictions


def predict(model, audio_path, label_map_reverse, device):
    """
    Load a WAV file, perform prediction and return results
    """
    # a. Set model to evaluation mode
    model.eval()

    # b. Prepare data transformation pipeline (must be exactly the same as during training)
    transform = torchaudio.transforms.MelSpectrogram(n_mels=128)
    target_length = 81

    # c. Load and transform audio
    waveform, _ = torchaudio.load(audio_path)
    spectrogram = transform(waveform)

    # d. Pad or truncate to target length (must be exactly the same as during training)
    current_length = spectrogram.shape[2]
    if current_length < target_length:
        padding_needed = target_length - current_length
        spectrogram = torch.nn.functional.pad(spectrogram, (0, padding_needed))
    elif current_length > target_length:
        spectrogram = spectrogram[:, :, :target_length]

    # e. Prepare input tensor for model
    #    - .unsqueeze(0) adds a dimension at the front to simulate a batch
    # input_tensor = spectrogram.unsqueeze(0).to(device)
    # New line (correct)
    input_tensor = spectrogram.to(device)

    # f. Perform prediction (in no_grad context to save computational resources)
    with torch.no_grad():
        predictions = model(input_tensor)

    # g. Interpret output
    #    - Softmax converts model output to probabilities
    probabilities = torch.nn.functional.softmax(predictions[0], dim=0)
    #    - Argmax finds the index of the class with highest probability
    predicted_index = torch.argmax(probabilities).item()
    #    - Use label_map_reverse to find corresponding word
    predicted_label = label_map_reverse[predicted_index]

    return predicted_label, probabilities[predicted_index].item()


def get_random_audio_files(data_path, num_files=10):
    """
    Get random audio files from the dataset
    """
    all_audio_files = []

    # Collect all wav files from all subdirectories
    for label_dir in os.listdir(data_path):
        label_path = os.path.join(data_path, label_dir)
        if (
            os.path.isdir(label_path)
            and not label_dir.startswith("_")
            and label_dir != "LICENSE"
        ):
            wav_files = glob.glob(os.path.join(label_path, "*.wav"))
            for wav_file in wav_files:
                # Store both file path and true label
                all_audio_files.append((wav_file, label_dir))

    # Randomly sample files
    random.shuffle(all_audio_files)
    return all_audio_files[:num_files]


if __name__ == "__main__":
    # --- Configuration ---
    # Path to your saved best model
    MODEL_PATH = (
        "/data/speech-recognition/runs/exp-20250913-205502/models/best-epoch15.pth"
    )
    # Dataset path
    DATA_PATH = "/data/SpeechCommands/speech_commands_v0.02"
    NUM_SAMPLES = 100  # Number of random audio files to test

    # Build label mapping
    full_data_path = DATA_PATH
    labels = []
    for item in os.listdir(full_data_path):
        item_path = os.path.join(full_data_path, item)
        if os.path.isdir(item_path) and not item.startswith("_") and item != "LICENSE":
            labels.append(item)

    # Sort labels for consistent mapping
    labels.sort()
    label_map_reverse = {idx: label for idx, label in enumerate(labels)}

    # --- Load model ---
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")

    # Create model instance and load weights
    model = AudioTransformer(num_classes=len(label_map_reverse)).to(device)
    model.load_state_dict(torch.load(MODEL_PATH, map_location=device))

    print(f"Model loaded from {MODEL_PATH}")
    print(f"Device: {device}")
    print(f"Testing {NUM_SAMPLES} random audio files...")
    print("=" * 80)

    # --- Get random audio files and run predictions ---
    random_files = get_random_audio_files(DATA_PATH, NUM_SAMPLES)

    correct_predictions = 0
    for i, (audio_path, true_label) in enumerate(random_files, 1):
        predicted_word, confidence = predict(
            model, audio_path, label_map_reverse, device
        )

        # Check if prediction is correct
        is_correct = predicted_word == true_label
        if is_correct:
            correct_predictions += 1

        status = "✓" if is_correct else "✗"

        print(f"[{i:2d}/{NUM_SAMPLES}] {status} File: {audio_path}")
        print(
            f"  True: '{true_label}' | Predicted: '{predicted_word}' | Confidence: {confidence:.2%}"
        )
        print()

    print("=" * 80)
    print(
        f"Accuracy: {correct_predictions}/{NUM_SAMPLES} ({correct_predictions/NUM_SAMPLES:.1%})"
    )
