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

import sys

import mlx.core as mx
import mlx.core.distributed as dist


def main() -> None:
    group = dist.init()
    rank = group.rank()
    world_size = group.size()

    if not 0 <= rank < world_size or world_size != 2:
        print(f"Unexpected rank/world size: {rank}/{world_size}", file=sys.stderr)
        sys.exit(1)

    global_sum = dist.all_sum(mx.array([float(rank)]), group=group)
    mx.eval(global_sum)

    expected_sum = sum(range(world_size))
    actual_sum = global_sum.item()
    if actual_sum != expected_sum:
        print(
            f"All-sum failed on rank {rank}: expected {expected_sum}, got {actual_sum}",
            file=sys.stderr,
        )
        sys.exit(1)


if __name__ == "__main__":
    main()
