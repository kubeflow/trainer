use std::env;
pub struct DatasetConfig {
    pub metadata_loc: String,
    pub schema_name: String,
    pub table_name: String,
}

impl DatasetConfig {
    pub fn from_env() -> Result<Self, Box<dyn std::error::Error>> {
        let metadata_loc = env::var("METADATA_LOC")?;
        let schema_name = env::var("SCHEMA_NAME")?;
        let table_name = env::var("TABLE_NAME")?;
        Ok(DatasetConfig {metadata_loc, schema_name, table_name})
    }
}