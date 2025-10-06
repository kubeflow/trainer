#!/bin/bash

echo "=== VERSION INFORMATION ==="
echo "Python version: $(python3 --version)"
echo "Pip version: $(pip --version)"
echo ""

echo "=== JAX Information ==="
python3 -c "import jax; print(f'JAX version: {jax.__version__}')"
python3 -c "import jaxlib; print(f'JAXLib version: {jaxlib.__version__}')"
echo ""

echo "=== NCCL Information ==="
if dpkg -l | grep -q libnccl2; then
    echo "NCCL version: $(dpkg -l | grep libnccl2 | awk '{print $3}')"
else
    echo "NCCL: Not available via package manager"
fi
if command -v nvcc &> /dev/null; then
    echo "CUDA version: $(nvcc --version | grep 'release' | awk '{print $5}')"
fi
echo ""

echo "=== TPU Information ==="
python3 -c "
try:
    import jax
    devices = jax.devices()
    tpu_devices = [d for d in devices if d.platform == 'tpu']
    if tpu_devices:
        print(f'TPU devices found: {len(tpu_devices)}')
        for d in tpu_devices:
            print(f'  - {d}')
    else:
        print('No TPU devices found')
except Exception as e:
    print(f'Error checking TPU devices: {e}')
"
echo ""

echo "=== Gloo Information ==="
echo "Gloo: Built from source (commit 43b7acbf372cdce14075f3526e39153b7e433b53)"
if [ -f /usr/local/lib/libgloo.a ]; then
    echo "Gloo library: Found at /usr/local/lib/libgloo.so"
else
    echo "Gloo library: Not found"
fi
echo ""

echo "=== Installed Python Packages ==="
pip list
