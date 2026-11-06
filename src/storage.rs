use anyhow::{Context, Result};
use fs2::FileExt;
use serde_json::Value;
use std::{
    fs::{self, OpenOptions},
    io::Write,
    path::Path,
};

pub fn read_json(path: &Path, fallback: Value) -> Result<Value> {
    match fs::read(path) {
        Ok(bytes) => {
            serde_json::from_slice(&bytes).with_context(|| format!("Read {}", path.display()))
        }
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(fallback),
        Err(error) => Err(error.into()),
    }
}

pub fn write_private(path: &Path, bytes: &[u8]) -> Result<()> {
    let parent = path
        .parent()
        .filter(|p| !p.as_os_str().is_empty())
        .unwrap_or(Path::new("."));
    fs::create_dir_all(parent)?;
    let mut temporary = tempfile::NamedTempFile::new_in(parent)?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        temporary
            .as_file()
            .set_permissions(fs::Permissions::from_mode(0o600))?;
    }
    temporary.write_all(bytes)?;
    temporary.as_file().sync_all()?;
    temporary.persist(path).map_err(|error| error.error)?;
    #[cfg(unix)]
    fs::File::open(parent)?.sync_all()?;
    Ok(())
}

pub fn write_json(path: &Path, value: &impl serde::Serialize) -> Result<()> {
    write_private(path, &serde_json::to_vec_pretty(value)?)
}

// Serialize read/modify/write across threads and separate CLI/daemon processes.
pub fn update_json(
    path: &Path,
    fallback: Value,
    update: impl FnOnce(&mut Value) -> Result<()>,
) -> Result<Value> {
    let parent = path.parent().unwrap_or(Path::new("."));
    fs::create_dir_all(parent)?;
    let mut options = OpenOptions::new();
    options.read(true).write(true).create(true).truncate(false);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let lock = options.open(path.with_extension("lock"))?;
    lock.lock_exclusive()?;
    let mut value = read_json(path, fallback)?;
    update(&mut value)?;
    write_json(path, &value)?;
    FileExt::unlock(&lock)?;
    Ok(value)
}
