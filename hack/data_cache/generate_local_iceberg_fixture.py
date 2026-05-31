#!/usr/bin/env python3
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

"""Generate a minimal on-disk Iceberg table for local data-cache development and CI.

Requires: pip install pyiceberg pyarrow sqlalchemy

Output layout (under pkg/data_cache/testdata/local_iceberg/warehouse):
  - Iceberg namespace: local
  - Iceberg table: demo
  - Columns: id (int), value (string) — no cache_index column

After generation, use METADATA_LOC pointing at the latest metadata JSON file.
"""

from __future__ import annotations

import argparse
import shutil
import sys
from pathlib import Path

import pyarrow as pa
from pyiceberg.catalog.sql import SqlCatalog
from pyiceberg.schema import Schema
from pyiceberg.types import IntegerType, NestedField, StringType


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    default_out = (
        Path(__file__).resolve().parents[2] / "pkg/data_cache/testdata/local_iceberg"
    )
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=default_out,
        help="Directory to write the warehouse and catalog DB",
    )
    args = parser.parse_args()

    output_dir: Path = args.output_dir
    warehouse = output_dir / "warehouse"
    if output_dir.exists():
        shutil.rmtree(output_dir)
    warehouse.mkdir(parents=True)

    warehouse_uri = warehouse.resolve().as_uri()
    catalog_db = output_dir / "catalog.db"

    catalog = SqlCatalog(
        "local_cache_fixture",
        **{
            "uri": f"sqlite:///{catalog_db}",
            "warehouse": warehouse_uri,
        },
    )

    schema = Schema(
        NestedField(field_id=1, name="id", field_type=IntegerType(), required=True),
        NestedField(field_id=2, name="value", field_type=StringType(), required=False),
    )

    catalog.create_namespace("local")
    table = catalog.create_table("local.demo", schema=schema)

    data = pa.Table.from_pydict(
        {
            "id": [1, 2, 3],
            "value": ["alpha", "beta", "gamma"],
        },
        schema=pa.schema(
            [
                pa.field("id", pa.int32(), nullable=False),
                pa.field("value", pa.string(), nullable=True),
            ]
        ),
    )
    table.append(data)

    metadata_files = sorted(
        (warehouse / "local" / "demo" / "metadata").glob("*.metadata.json")
    )
    if not metadata_files:
        print("ERROR: no metadata.json produced", file=sys.stderr)
        return 1

    latest = metadata_files[-1].resolve().as_uri()
    print(f"Generated local Iceberg table at {warehouse}")
    print("SCHEMA_NAME=local TABLE_NAME=demo")
    print(f"METADATA_LOC={latest}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
