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

def get_num_nodes(num_slices: int, topology: str, chips_per_host: int = 4) -> int:
    """
    Compute the total number of nodes (hosts) required across all TPU slices.

    This utility function assists in configuring multi-slice TPU TrainJobs where the 
    user wants to scale their training job across multiple physical TPU slices. It calculates 
    the aggregate `num_nodes` based on the slice count and physical TPU topology dimensions.

    Args:
        num_slices: The number of TPU slices.
        topology: The TPU topology string (e.g., "2x2", "2x4", "2x2x2", "2x2x4", "4x4").
        chips_per_host: The number of TPU chips per VM host node. Defaults to 4.

    Returns:
        The total number of nodes (hosts) across all slices.

    Raises:
        ValueError: If topology is empty, improperly formatted, or if the total chips 
                    in a slice is not evenly divisible by the chips_per_host.
    """
    if not topology:
        raise ValueError("TPU topology must be specified.")

    # Parse the topology dimensions (e.g. "2x2" or "2x2x2")
    try:
        dims = [int(d) for d in topology.lower().split("x")]
    except ValueError:
        raise ValueError(
            f"Invalid topology format: '{topology}'. Must be formatted as 'AxB' or 'AxBxC' (e.g. '2x2', '2x2x2')."
        )

    # Calculate total TPU chips in a single slice
    chips_per_slice = 1
    for dim in dims:
        chips_per_slice *= dim

    # Validate that the topology can be cleanly allocated into VM hosts
    if chips_per_slice % chips_per_host != 0:
        raise ValueError(
            f"Total TPU chips in slice ({chips_per_slice}) must be cleanly divisible by chips_per_host ({chips_per_host})."
        )

    hosts_per_slice = chips_per_slice // chips_per_host
    return num_slices * hosts_per_slice
