import torchaudio

print("Downloading SpeechCommands dataset...")

# This command will download the data to a folder named "SpeechCommands"
# in your current directory if it's not already there.
train_dataset = torchaudio.datasets.SPEECHCOMMANDS(root="/data", download=True)

print("Download complete!")
print(f"Number of training samples: {len(train_dataset)}")
