# Copyright The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import unittest
from kubeflow_trainer_api.tpu import get_num_nodes

class TestTPUUtils(unittest.TestCase):
    def test_get_num_nodes_v5e(self):
        # TPU v5e single-slice configurations
        self.assertEqual(get_num_nodes(num_slices=1, topology="2x2"), 1)
        self.assertEqual(get_num_nodes(num_slices=1, topology="2x4"), 2)
        self.assertEqual(get_num_nodes(num_slices=1, topology="4x4"), 4)
        
        # TPU v5e multi-slice scaling
        self.assertEqual(get_num_nodes(num_slices=2, topology="2x2"), 2)
        self.assertEqual(get_num_nodes(num_slices=4, topology="2x4"), 8)
        self.assertEqual(get_num_nodes(num_slices=2, topology="4x4"), 8)

    def test_get_num_nodes_v4_v5p(self):
        # TPU v4/v5p 3D topologies
        self.assertEqual(get_num_nodes(num_slices=1, topology="2x2x2"), 2)
        self.assertEqual(get_num_nodes(num_slices=3, topology="2x2x2"), 6)
        self.assertEqual(get_num_nodes(num_slices=1, topology="2x2x4"), 4)
        self.assertEqual(get_num_nodes(num_slices=2, topology="2x2x4"), 8)

    def test_case_insensitivity(self):
        self.assertEqual(get_num_nodes(num_slices=2, topology="2X4"), 4)
        self.assertEqual(get_num_nodes(num_slices=1, topology="2X2X2"), 2)

    def test_custom_chips_per_host(self):
        # Custom chip VM host overrides (e.g. 8 chips per host)
        self.assertEqual(get_num_nodes(num_slices=2, topology="4x4", chips_per_host=8), 4)

    def test_invalid_topologies(self):
        # Assert ValueError for invalid formats
        with self.assertRaises(ValueError):
            get_num_nodes(num_slices=1, topology="")
        with self.assertRaises(ValueError):
            get_num_nodes(num_slices=1, topology="invalid-topo")
        with self.assertRaises(ValueError):
            get_num_nodes(num_slices=1, topology="2x")

        # Assert ValueError for indivisible layouts
        with self.assertRaises(ValueError):
            get_num_nodes(num_slices=1, topology="2x3")  # 6 chips not divisible by 4

if __name__ == "__main__":
    unittest.main()
