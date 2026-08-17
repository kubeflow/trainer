FROM python:3.8

# Pinned to the versions this example shipped with in v1.9.3. Left unpinned, pip
# resolves a typing-extensions that no longer publishes a cp38 wheel and falls
# back to its sdist; building that needs flit_core, which --index-url cannot
# reach because it replaces PyPI rather than adding to it.
# torch is pinned without the +cpu local version so the same pin resolves on
# both linux/amd64 (2.4.1+cpu) and linux/arm64 (2.4.1).
RUN pip3 install \
      torch==2.4.1 \
      torchvision==0.19.1 \
      torchaudio==2.4.1 \
      typing_extensions==4.9.0 \
      --index-url https://download.pytorch.org/whl/cpu

WORKDIR /

COPY demo.py .
