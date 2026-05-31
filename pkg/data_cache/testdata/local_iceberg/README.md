# Local Iceberg fixture for data cache

This directory holds a generated on-disk Iceberg table used for local development and CI.

**Do not commit generated files** (warehouse, `catalog.db`). Regenerate with:

```bash
python3 hack/data_cache/generate_local_iceberg_fixture.py
```

Requires: `pip install pyiceberg pyarrow sqlalchemy`

After generation:

- `SCHEMA_NAME=local`
- `TABLE_NAME=demo`
- `METADATA_LOC` — printed by the script (latest `*.metadata.json` under `warehouse/local/demo/metadata/`)
