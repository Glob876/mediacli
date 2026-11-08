//! Bundled media tools and the separately updated official yt-dlp executable.
use anyhow::{Context, Result, bail, ensure};
use fs2::FileExt;
use serde_json::{Value, json};
use sha2::{Digest, Sha256};
use std::{
    fs,
    io::Write,
    path::{Path, PathBuf},
    time::{Duration, SystemTime, UNIX_EPOCH},
};

pub fn filename(name: &str) -> String {
    if cfg!(windows) {
        format!("{name}.exe")
    } else {
        name.into()
    }
}

pub fn bundled(name: &str) -> Option<PathBuf> {
    let executable = std::env::current_exe().ok()?;
    let directory = executable.parent()?;
    let candidate = directory.join(filename(name));
    candidate.is_file().then_some(candidate)
}

fn directory() -> PathBuf {
    dirs::data_local_dir()
        .unwrap_or_else(crate::config::directory)
        .join("mediacli/tools/yt-dlp")
}

pub fn installed_in(directory: &Path) -> Option<PathBuf> {
    let receipt = crate::storage::read_json(&directory.join("current.json"), json!({})).ok()?;
    let hash = receipt["sha256"].as_str()?;
    if hash.len() != 64 || !hash.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return None;
    }
    let candidate = directory.join(hash).join(filename("yt-dlp"));
    candidate.is_file().then_some(candidate)
}

pub fn installed() -> Option<PathBuf> {
    installed_in(&directory())
}

pub fn release_asset(os: &str, arch: &str) -> Result<&'static str> {
    match (os, arch) {
        ("windows", "x86_64") => Ok("yt-dlp.exe"),
        ("windows", "aarch64") => Ok("yt-dlp_arm64.exe"),
        ("linux", "x86_64") => Ok("yt-dlp_linux"),
        ("linux", "aarch64") => Ok("yt-dlp_linux_aarch64"),
        ("macos", "x86_64" | "aarch64") => Ok("yt-dlp_macos"),
        _ => bail!("No official yt-dlp executable for {os}/{arch}"),
    }
}

fn official_url(raw: &str) -> Result<reqwest::Url> {
    let url = reqwest::Url::parse(raw)?;
    ensure!(
        url.scheme() == "https"
            && url.host_str() == Some("github.com")
            && url.path().starts_with("/yt-dlp/yt-dlp/releases/download/"),
        "Unexpected yt-dlp release URL"
    );
    Ok(url)
}

fn checksum_for<'a>(manifest: &'a str, asset: &str) -> Result<&'a str> {
    let expected = manifest
        .lines()
        .find_map(|line| {
            let (hash, name) = line.split_once(char::is_whitespace)?;
            (name.trim().trim_start_matches('*') == asset).then_some(hash)
        })
        .context("yt-dlp checksum missing from official release")?;
    ensure!(
        expected.len() == 64 && expected.bytes().all(|byte| byte.is_ascii_hexdigit()),
        "Invalid yt-dlp checksum"
    );
    Ok(expected)
}

pub fn verify_checksum(bytes: &[u8], manifest: &str, asset: &str) -> Result<String> {
    let expected = checksum_for(manifest, asset)?;
    let actual = format!("{:x}", Sha256::digest(bytes));
    ensure!(actual == expected, "yt-dlp SHA-256 checksum mismatch");
    Ok(actual)
}

async fn response_bytes(
    client: &reqwest::Client,
    url: reqwest::Url,
    limit: usize,
) -> Result<Vec<u8>> {
    let mut response = client.get(url).send().await?.error_for_status()?;
    if let Some(length) = response.content_length() {
        ensure!(length <= limit as u64, "Download exceeds size limit");
    }
    let mut bytes = Vec::new();
    while let Some(chunk) = response.chunk().await? {
        ensure!(
            bytes.len() + chunk.len() <= limit,
            "Download exceeds size limit"
        );
        bytes.extend_from_slice(&chunk);
    }
    Ok(bytes)
}

async fn update_in(directory: &Path) -> Result<PathBuf> {
    let asset_name = release_asset(std::env::consts::OS, std::env::consts::ARCH)?;
    let client = reqwest::Client::builder()
        .user_agent(concat!("MediaCLI/", env!("CARGO_PKG_VERSION")))
        .https_only(true)
        .connect_timeout(Duration::from_secs(20))
        .timeout(Duration::from_secs(180))
        .build()?;
    let metadata = response_bytes(
        &client,
        reqwest::Url::parse("https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest")?,
        4 * 1024 * 1024,
    )
    .await?;
    let release: Value = serde_json::from_slice(&metadata)?;
    ensure!(
        release["draft"] == false && release["prerelease"] == false,
        "Expected a stable yt-dlp release"
    );
    let assets = release["assets"]
        .as_array()
        .context("Invalid GitHub release")?;
    let url_for = |name: &str| -> Result<reqwest::Url> {
        let asset = assets
            .iter()
            .find(|asset| asset["name"] == name)
            .context("Required file missing from yt-dlp release")?;
        official_url(
            asset["browser_download_url"]
                .as_str()
                .context("Missing asset URL")?,
        )
    };
    let manifest_bytes = response_bytes(&client, url_for("SHA2-256SUMS")?, 1024 * 1024).await?;
    let manifest = std::str::from_utf8(&manifest_bytes)?;
    let hash = checksum_for(manifest, asset_name)?.to_owned();
    let version_directory = directory.join(&hash);
    fs::create_dir_all(&version_directory)?;
    let destination = version_directory.join(filename("yt-dlp"));
    let valid = fs::read(&destination)
        .is_ok_and(|bytes| verify_checksum(&bytes, manifest, asset_name).is_ok());
    if !valid {
        let executable = response_bytes(&client, url_for(asset_name)?, 256 * 1024 * 1024).await?;
        verify_checksum(&executable, manifest, asset_name)?;
        let mut temporary = tempfile::Builder::new()
            .suffix(if cfg!(windows) { ".exe" } else { "" })
            .tempfile_in(&version_directory)?;
        temporary.write_all(&executable)?;
        temporary.as_file().sync_all()?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            temporary
                .as_file()
                .set_permissions(fs::Permissions::from_mode(0o700))?;
        }
        let temporary = temporary.into_temp_path();
        let output = tokio::time::timeout(
            Duration::from_secs(20),
            tokio::process::Command::new(&temporary)
                .arg("--version")
                .kill_on_drop(true)
                .output(),
        )
        .await??;
        ensure!(
            output.status.success(),
            "Downloaded yt-dlp failed its version check"
        );
        temporary
            .persist(&destination)
            .map_err(|error| error.error)?;
    }
    // Immutable version paths keep running downloads alive, including on Windows.
    crate::storage::write_json(
        &directory.join("current.json"),
        &json!({"sha256":hash,"release":release["tag_name"],"checked_at":now()}),
    )?;
    Ok(destination)
}

fn now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

pub async fn ensure_ytdlp(force: bool) -> Result<PathBuf> {
    let directory = directory();
    fs::create_dir_all(&directory)?;
    let lock_path = directory.join("update.lock");
    let lock = tokio::task::spawn_blocking(move || -> Result<fs::File> {
        let file = fs::OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .open(lock_path)?;
        file.lock_exclusive()?;
        Ok(file)
    })
    .await??;
    let current = installed_in(&directory);
    let receipt = crate::storage::read_json(&directory.join("current.json"), json!({}))?;
    if !force
        && let Some(ref path) = current
        && now().saturating_sub(receipt["checked_at"].as_u64().unwrap_or(0)) < 86400
    {
        return Ok(path.clone());
    }
    eprintln!("MediaCLI: checking the official yt-dlp GitHub release…");
    let result = update_in(&directory).await;
    FileExt::unlock(&lock)?;
    match result {
        Ok(path) => Ok(path),
        Err(error) if !force && current.is_some() => {
            eprintln!("MediaCLI: yt-dlp update unavailable ({error}); using the installed version.");
            Ok(current.unwrap())
        }
        Err(error) => Err(error.context("Could not install yt-dlp from its official GitHub release; retry with mediacli tools update")),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn checksums_reject_tampering_missing_assets_and_partial_names() {
        let bytes = b"official executable";
        let hash = format!("{:x}", Sha256::digest(bytes));
        let manifest = format!("{hash}  yt-dlp.exe\n");
        assert_eq!(
            verify_checksum(bytes, &manifest, "yt-dlp.exe").unwrap(),
            hash
        );
        assert!(verify_checksum(b"modified", &manifest, "yt-dlp.exe").is_err());
        assert!(verify_checksum(bytes, &manifest, "yt-dlp").is_err());
        assert!(verify_checksum(bytes, "../escape yt-dlp.exe", "yt-dlp.exe").is_err());
    }

    #[test]
    fn release_links_reject_other_repositories_and_unencrypted_downloads() {
        assert!(
            official_url(
                "https://github.com/yt-dlp/yt-dlp/releases/download/2026.10.26/yt-dlp.exe"
            )
            .is_ok()
        );
        for url in [
            "http://github.com/yt-dlp/yt-dlp/releases/download/x/a",
            "https://github.com/other/project/releases/download/x/a",
            "https://github.com.evil.example/yt-dlp/yt-dlp/releases/download/x/a",
        ] {
            assert!(official_url(url).is_err());
        }
    }

    #[test]
    fn managed_receipts_cannot_escape_the_tools_directory() {
        let directory = tempfile::tempdir().unwrap();
        crate::storage::write_json(
            &directory.path().join("current.json"),
            &json!({"sha256":"../outside"}),
        )
        .unwrap();
        assert!(installed_in(directory.path()).is_none());
        let hash = "a".repeat(64);
        fs::create_dir(directory.path().join(&hash)).unwrap();
        fs::write(
            directory.path().join(&hash).join(filename("yt-dlp")),
            b"tool",
        )
        .unwrap();
        crate::storage::write_json(
            &directory.path().join("current.json"),
            &json!({"sha256":hash}),
        )
        .unwrap();
        assert!(
            installed_in(directory.path())
                .unwrap()
                .starts_with(directory.path())
        );
    }

    #[test]
    fn supported_targets_use_standalone_executables_without_python() {
        for (os, arch, asset) in [
            ("windows", "x86_64", "yt-dlp.exe"),
            ("windows", "aarch64", "yt-dlp_arm64.exe"),
            ("linux", "x86_64", "yt-dlp_linux"),
            ("linux", "aarch64", "yt-dlp_linux_aarch64"),
            ("macos", "aarch64", "yt-dlp_macos"),
        ] {
            assert_eq!(release_asset(os, arch).unwrap(), asset);
        }
        assert!(release_asset("linux", "x86").is_err());
    }
}
