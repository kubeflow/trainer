
# loading libraries
load('ext://restart_process', 'docker_build_with_restart')

allow_k8s_contexts('kind-trainer-dev')

# Allow overriding the local registry port via REGISTRY_PORT env var
registry_port = os.getenv('REGISTRY_PORT', '5001')
registry = "localhost:%s" % registry_port

default_registry('localhost:5001')

# helper to build full image names
def img(name, tag = "latest"):
    return "%s/%s:%s" % (registry, name, tag)

# Docker images
os.environ['DOCKER_BUILDKIT']  = '0'
docker_build_with_restart(img("trainer/trainer-controller-manager"), ".", entrypoint="/manager", dockerfile="cmd/trainer-controller-manager/Dockerfile", live_update=[sync('./pkg', '/manager')])
docker_build(img("trainer/dataset-initializer"), ".", dockerfile="cmd/initializers/dataset/Dockerfile")
docker_build(img("trainer/model-initializer"), ".", dockerfile="cmd/initializers/model/Dockerfile")

# Local resource to generate manifests
# @szaher change generate output directories so we can add deps
local_resource('generate-manifests', 'make generate')

# Load Kubernetes YAMLs
# ——— Load each overlay and apply it ———
manager_yaml = kustomize('manifests/overlays/local')
k8s_yaml(manager_yaml)

runtime_yaml = kustomize('manifests/overlays/runtimes')
k8s_yaml(runtime_yaml)

# ——— Dynamically carve out every object as its own resource ———
all_overlays = [runtime_yaml]

for overlay in all_overlays:
  for obj in decode_yaml_stream(overlay):
    nm = obj['metadata']['name']
    kd = obj['kind']
    ap = obj['apiVersion']

    # Print for debugging (you’ll see this in your Tilt logs)
    print("Registering Tilt resource: %s %s/%s" % (nm, kd, ap))

    # struct(name=nm, kind=kd, apiVersion=ap)
    selector = "%s:%s" % (nm, kd)

    k8s_resource(
      new_name      = nm,
      objects       = [selector],
      resource_deps = ['generate-manifests'],
    )
# Ignore docs and tests in file watching
watch_settings(ignore=['docs/**', 'test/**'])
