# Copyright 2019 kubeflow.org.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import setuptools

TESTS_REQUIRES = [
    "pytest",
    "pytest-tornasync",
    "mypy",
    "black==24.3.0",
    "flake8==4.0.1",
]

REQUIRES = [
    "certifi>=14.05.14",
    "six>=1.10",
    "setuptools>=21.0.0",
    "urllib3>=1.15.1",
    "kubernetes>=27.2.0",
    "retrying>=1.3.3",
]

setuptools.setup(
    name="kubeflow-training",
    version="1.9.2",
    author="Kubeflow Authors",
    author_email="hejinchi@cn.ibm.com",
    license="Apache License Version 2.0",
    url="https://github.com/kubeflow/training-operator/tree/master/sdk/python",
    description="Training Operator Python SDK",
    long_description="Training Operator Python SDK",
    packages=setuptools.find_packages(include=("kubeflow*")),
    package_data={},
    include_package_data=False,
    zip_safe=False,
    classifiers=[
        "Intended Audience :: Developers",
        "Intended Audience :: Education",
        "Intended Audience :: Science/Research",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "License :: OSI Approved :: Apache Software License",
        "Operating System :: OS Independent",
        "Topic :: Scientific/Engineering",
        "Topic :: Scientific/Engineering :: Artificial Intelligence",
        "Topic :: Software Development",
        "Topic :: Software Development :: Libraries",
        "Topic :: Software Development :: Libraries :: Python Modules",
    ],
    install_requires=REQUIRES,
    tests_require=TESTS_REQUIRES,
    extras_require={
        "test": TESTS_REQUIRES,
        "huggingface": ["transformers==4.50.2", "peft==0.15.1"],
    },
)
