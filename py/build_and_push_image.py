#!/usr/bin/env python
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# This is a helper script for building Docker images.
import argparse
import hashlib
import logging
import os
import shutil
import subprocess
import tempfile

import jinja2


def GetGitHash():
  # The image tag is based on the githash.
  git_hash = subprocess.check_output(["git", "rev-parse", "--short", "HEAD"])
  git_hash = git_hash.strip()
  modified_files = subprocess.check_output(["git", "ls-files", "--modified"])
  untracked_files = subprocess.check_output(["git", "ls-files", "--others",
                                             "--exclude-standard"])
  if modified_files or untracked_files:
    diff = subprocess.check_output(["git", "diff"])
    sha = hashlib.sha256()
    sha.update(diff)
    diffhash = sha.hexdigest()[0:7]
    git_hash = "{0}-dirty-{1}".format(git_hash, diffhash)
  return git_hash


def run_and_stream(cmd):
  logging.info("Running %s", " ".join(cmd))
  process = subprocess.Popen(cmd, stdout=subprocess.PIPE,
                             stderr=subprocess.STDOUT)

  while process.poll() is None:
    process.stdout.flush()
    for line in iter(process.stdout.readline, ''):
      logging.info(line.strip())

  process.stdout.flush()
  for line in iter(process.stdout.readline, ''):
    logging.info(line.strip())

  if process.returncode != 0:
    raise ValueError("cmd: {0} exited with code {1}".format(
        " ".join(cmd), process.returncode))


def build_and_push(dockerfile_template, image, modes=None,
                   skip_push=False, base_images=None):
  """Build and push images based on a Dockerfile template.

  Args:
    dockerfile_template: Path to a Dockerfile which can be a jinja2
      template that uses {{base_image}} as a place holder for the base
      image.
    image: The URI for the image to produce e.g. gcr.io/my-registry/my-image.
      This should not include a tag. Tag will be computed automatically
      based on the git hash of the directory.
     modes: This should be a list of keys corresponding to a subset of
       base_images. If none it will default to all keys in base_images.
       One image is built for each value in modes by using the associated
       value in base_images as the base image for the Dockerfile.
     skip_push: Whether to skip the push.
     base_images: A dictionary that maps modes to base images.

  Returns:
     images: A dictionary mapping modes to the corresponding docker
       image that was built.
  """
  if not modes:
    modes = base_images.keys()

  loader = jinja2.FileSystemLoader(os.path.dirname(dockerfile_template))

  if not base_images:
    raise ValueError("base_images must be provided.")

  images = {}
  for mode in modes:
    dockerfile_contents = jinja2.Environment(loader=loader).get_template(
        os.path.basename(dockerfile_template)).render(base_image=base_images[mode])
    context_dir = tempfile.mkdtemp(prefix="tmpTfJobSampleContentxt")
    logging.info("context_dir: %s", context_dir)
    shutil.rmtree(context_dir)
    shutil.copytree(os.path.dirname(dockerfile_template), context_dir)
    dockerfile = os.path.join(context_dir, 'Dockerfile')
    with open(dockerfile, 'w') as hf:
      hf.write(dockerfile_contents)

    full_image = image + "-" + mode

    full_image += ":" + GetGitHash()

    run_and_stream(["docker", "build", "-t", full_image, context_dir])
    logging.info("Built image: %s", full_image)

    images[mode] = full_image
    if not skip_push:
      if "gcr.io" in full_image:
        run_and_stream(["gcloud", "docker", "--", "push", full_image])
      else:
        run_and_stream(["docker", "--", "push", full_image])
      logging.info("Pushed image: %s", full_image)
  return images


def main():
  logging.getLogger().setLevel(logging.INFO)
  parser = argparse.ArgumentParser(
      description="Build Docker images based off of TensorFlow.")

  parser.add_argument(
      "--image",
      default="gcr.io/tf-on-k8s-dogfood",
      type=str,
      help="The image path to use; mode will be applied as a suffix.")

  parser.add_argument(
      "--dockerfile",
      required=True,
      type=str,
      help="The path to the Dockerfile")

  # TODO(jlewi): Should we make this a list so we can build both images with one command.
  parser.add_argument(
      '--mode',
      default=["cpu", "gpu"],
      dest="modes",
      action="append",
      help='Which image to build; options are cpu or gpu')

  parser.add_argument("--no-push", dest="should_push", action="store_false",
                      help="Do not push the image once build is finished.")

  args = parser.parse_args()

  base_images = {
      "cpu": "gcr.io/tensorflow/tensorflow:1.3.0",
      "gpu": "gcr.io/tensorflow/tensorflow:1.3.0-gpu",
  }

  build_and_push(dockerfile_template=args.dockerfile, modes=args.modes,
                 image=args.image,
                 skip_push=(not args.should_push),
                 base_images=base_images)


if __name__ == "__main__":
  main()
