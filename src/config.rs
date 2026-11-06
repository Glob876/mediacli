use crate::storage;
use anyhow::{Context, Result, bail};
use serde_json::{Value, json};
use std::path::{Path, PathBuf};

pub fn directory() -> PathBuf {
    dirs::config_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join("mediacli")
}

pub fn user_path(raw: &str) -> PathBuf {
    let text = raw.trim();
    let text = if text.len() >= 2
        && ((text.starts_with('"') && text.ends_with('"'))
            || (text.starts_with('\'') && text.ends_with('\'')))
    {
        &text[1..text.len() - 1]
    } else {
        text
    };
    if text == "~" {
        return dirs::home_dir().unwrap_or_default();
    }
    if let Some(relative) = text.strip_prefix("~/").or_else(|| text.strip_prefix("~\\")) {
        return dirs::home_dir().unwrap_or_default().join(relative);
    }
    PathBuf::from(if text.is_empty() { "." } else { text })
}

pub fn defaults() -> Value {
    let mut value: Value =
        serde_json::from_str(include_str!("../resources/config.json")).expect("embedded defaults");
    let home = dirs::home_dir().unwrap_or_default();
    value["download_dir"] = json!(home.join("Downloads/MediaCLI"));
    value["cookies_file"] = json!(directory().join("cookies.txt"));
    value["archive_file"] = json!(directory().join("archive.txt"));
    value["ffprobe_path"] = json!("");
    value["ffmpeg_path"] = json!("");
    value
}

pub fn load() -> Result<Value> {
    let mut value = defaults();
    let saved = storage::read_json(&directory().join("config.json"), json!({}))?;
    merge(&mut value, &saved)?;
    Ok(value)
}

pub fn merge(destination: &mut Value, source: &Value) -> Result<()> {
    let object = source.as_object().context("expected JSON object")?;
    let target = destination
        .as_object_mut()
        .context("expected JSON object")?;
    for (key, value) in object {
        target.insert(key.clone(), value.clone());
    }
    Ok(())
}

pub fn validate(value: &Value) -> Result<()> {
    let initial = defaults();
    for (key, item) in value.as_object().context("expected configuration object")? {
        let default = initial
            .get(key)
            .with_context(|| format!("unknown setting: {key}"))?;
        let valid = match default {
            Value::Bool(_) => item.is_boolean(),
            Value::String(_) => item.is_string(),
            Value::Number(_) => item.is_i64(),
            Value::Array(_) => item.is_array(),
            Value::Object(_) => item.is_object(),
            _ => true,
        };
        if !valid {
            bail!("invalid type for {key}");
        }
    }
    for (key, minimum, maximum) in [("bg_queue_max", 1, 32), ("concurrent_fragments", 1, 64)] {
        let number = value[key].as_i64().context("integer required")?;
        if !(minimum..=maximum).contains(&number) {
            bail!("{key} must be between {minimum} and {maximum}");
        }
    }
    let choices = metadata();
    for (key, group) in [
        ("theme", "themes"),
        ("audio_format", "audio_formats"),
        ("thumbnail_format", "thumbnail_format"),
        ("cookies_browser", "browsers"),
        ("progress_style", "progress_styles"),
        ("user_goal", "user_goals"),
        ("logo_protocol", "logo_protocols"),
        ("logo_ascii_preset", "logo_ascii"),
    ] {
        let selected = string(value, key);
        if !choices[group]
            .as_array()
            .unwrap()
            .iter()
            .any(|item| item.as_str() == Some(&selected) || item["id"].as_str() == Some(&selected))
        {
            bail!("invalid {key}: {selected}");
        }
    }
    for (key, choices) in [
        ("language", vec!["ru", "en", "en-US", "en-GB"]),
        ("cookies_mode", vec!["none", "browser", "file"]),
        ("proxy_mode", vec!["system", "none", "custom"]),
        ("transcode_mode", vec!["embedded", "external"]),
        ("logo_mode", vec!["ascii", "image", "none"]),
    ] {
        if !choices.contains(&string(value, key).as_str()) {
            bail!("invalid {key}");
        }
    }
    let accent = string(value, "accent_color");
    if accent.len() != 7
        || !accent.starts_with('#')
        || !accent[1..].chars().all(|c| c.is_ascii_hexdigit())
    {
        bail!("accent_color must be #RRGGBB");
    }
    if string(value, "download_dir").trim().is_empty() {
        bail!("download_dir cannot be empty");
    }
    Ok(())
}

pub fn save(value: &Value) -> Result<()> {
    validate(value)?;
    let path = directory().join("config.json");
    if path.exists()
        && !std::fs::read_dir(directory())?
            .filter_map(|e| e.ok())
            .any(|e| {
                e.file_name()
                    .to_string_lossy()
                    .starts_with("config.json.bak-")
            })
    {
        storage::write_private(
            &directory().join(format!("config.json.bak-{}", crate::media::timestamp())),
            &std::fs::read(&path)?,
        )?;
    }
    storage::write_json(&path, value)
}
pub fn metadata() -> Value {
    serde_json::from_str(include_str!("../resources/meta.json")).unwrap()
}
pub fn download_fields() -> Value {
    serde_json::from_str(include_str!("../resources/download_fields.json")).unwrap()
}
pub fn video_presets() -> Value {
    serde_json::from_str::<Value>(include_str!("../resources/presets.json")).unwrap()["video_presets"].clone()
}
pub fn convert_presets() -> Value {
    serde_json::from_str::<Value>(include_str!("../resources/convert_presets.json")).unwrap()["convert_presets"].clone()
}
pub fn schema() -> Value {
    serde_json::from_str(include_str!("../resources/operation_schema.json")).unwrap()
}
pub fn string(value: &Value, key: &str) -> String {
    match &value[key] {
        Value::String(text) => text.clone(),
        Value::Number(number) => number.to_string(),
        _ => String::new(),
    }
}
pub fn boolean(value: &Value, key: &str) -> bool {
    value[key].as_bool().unwrap_or(false)
}
pub fn number(value: &Value, key: &str, fallback: f64) -> f64 {
    value[key]
        .as_f64()
        .or_else(|| value[key].as_str().and_then(|s| s.parse().ok()))
        .unwrap_or(fallback)
}
pub fn tool(value: &Value, name: &str) -> String {
    let configured = string(value, &format!("{name}_path"));
    if configured.is_empty() {
        if name == "ffprobe" {
            let ffmpeg = string(value, "ffmpeg_path");
            let sibling = Path::new(&ffmpeg).with_file_name(if cfg!(windows) {
                "ffprobe.exe"
            } else {
                "ffprobe"
            });
            if !ffmpeg.is_empty() && sibling.is_file() {
                return sibling.to_string_lossy().into_owned();
            }
        }
        name.into()
    } else {
        user_path(&configured).to_string_lossy().into_owned()
    }
}

pub fn history() -> Result<Value> {
    storage::read_json(&directory().join("history.json"), json!([]))
}
pub fn media_presets() -> Result<Value> {
    storage::read_json(&directory().join("media_presets.json"), json!([]))
}
pub fn add_history(entry: Value) -> Result<()> {
    storage::update_json(&directory().join("history.json"), json!([]), |history| {
        history
            .as_array_mut()
            .context("invalid history")?
            .insert(0, entry);
        Ok(())
    })?;
    Ok(())
}
