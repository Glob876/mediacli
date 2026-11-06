use crate::{
    cli::{self, Arguments},
    config::{self, string},
    media::{self, Pipeline},
    storage,
};
use anyhow::{Context, Result, ensure};
use serde_json::{Value, json};
fn action(args: &Arguments) -> &str {
    args.positionals
        .first()
        .map(String::as_str)
        .unwrap_or("list")
}
fn escaped(value: &str) -> String {
    reqwest::Url::parse("http://localhost/")
        .unwrap()
        .query_pairs_mut()
        .append_pair("value", value)
        .finish()
        .query()
        .unwrap()
        .trim_start_matches("value=")
        .to_string()
}
pub async fn execute(command: &str, args: &Arguments) -> Result<bool> {
    if cli::server(args).is_empty() {
        return Ok(false);
    }
    let compact = args.boolean("json");
    match command {
        "doctor" => cli::emit(
            &cli::request(args, "GET", "/api/status", None).await?,
            compact,
        )?,
        "library" => {
            let path = if let Some(directory) = args.positionals.first() {
                format!("/api/browse?path={}", escaped(directory))
            } else {
                "/api/library".into()
            };
            cli::emit(&cli::request(args, "GET", &path, None).await?, compact)?;
        }
        "probe" | "inspect" => {
            let input = args
                .positionals
                .first()
                .cloned()
                .unwrap_or(args.value("input"));
            cli::emit(
                &cli::request(
                    args,
                    "GET",
                    &format!("/api/probe?path={}", escaped(&input)),
                    None,
                )
                .await?,
                compact,
            )?;
        }
        "cookies" => return Err(cli::usage("cookies manages a local file; omit --server")),
        "config" => {
            let mut cfg = cli::request(args, "GET", "/api/config", None).await?;
            match action(args) {
                "list" | "get" => cli::emit(
                    if let Some(key) = args.positionals.get(1) {
                        cfg.get(key).context("unknown setting")?
                    } else {
                        &cfg
                    },
                    compact,
                )?,
                "options" => cli::emit(
                    &cli::request(args, "GET", "/api/meta", None).await?,
                    compact,
                )?,
                "reset" => {
                    if args.boolean("dry-run") {
                        cli::emit(&config::defaults(), compact)?;
                    } else {
                        cli::emit(
                            &cli::request(args, "POST", "/api/config/reset", Some(json!({})))
                                .await?,
                            compact,
                        )?;
                    }
                }
                "export" => {
                    let path = args.positionals.get(1).context("file required")?;
                    if !args.boolean("dry-run") {
                        storage::write_json(&config::user_path(path), &cfg)?;
                    }
                }
                "set" | "import" => {
                    if action(args) == "set" {
                        let key = args.positionals.get(1).context("setting required")?;
                        let raw = args.positionals.get(2).context("value required")?;
                        let old = cfg.get(key).context("unknown setting")?;
                        cfg[key] = if old.is_string() {
                            json!(raw)
                        } else {
                            serde_json::from_str(raw)?
                        };
                    } else {
                        let path = args.positionals.get(1).context("file required")?;
                        let imported = storage::read_json(&config::user_path(path), Value::Null)?;
                        config::merge(&mut cfg, &imported)?;
                    }
                    config::validate(&cfg)?;
                    if args.boolean("dry-run") {
                        cli::emit(&cfg, compact)?;
                    } else {
                        cli::emit(
                            &cli::request(args, "PUT", "/api/config", Some(cfg)).await?,
                            compact,
                        )?;
                    }
                }
                _ => return Err(cli::usage("unknown config action")),
            }
        }
        "history" => {
            let response = cli::request(args, "GET", "/api/history", None).await?;
            let entries = response["history"].as_array().context("invalid history")?;
            match action(args) {
                "list" => {
                    let query = args.value("search").to_lowercase();
                    let values: Vec<_> = entries
                        .iter()
                        .enumerate()
                        .filter(|(_, e)| e.to_string().to_lowercase().contains(&query))
                        .map(|(index, entry)| {
                            let mut value = entry.clone();
                            value["index"] = json!(index);
                            value
                        })
                        .collect();
                    cli::emit(&json!(values), compact)?;
                }
                "clear" => {
                    ensure!(
                        args.boolean("yes") || args.boolean("y"),
                        "use --yes to clear history"
                    );
                    if !args.boolean("dry-run") {
                        cli::emit(
                            &cli::request(args, "DELETE", "/api/history", None).await?,
                            compact,
                        )?;
                    }
                }
                other => {
                    let index: usize = args
                        .positionals
                        .get(1)
                        .context("history index required")?
                        .parse()?;
                    let entry = entries.get(index).context("history index out of range")?;
                    match other {
                        "show" => cli::emit(entry, compact)?,
                        "delete" => {
                            if !args.boolean("dry-run") {
                                let mut value = entry.clone();
                                value["delete_file"] = json!(args.boolean("delete-file"));
                                cli::emit(
                                    &cli::request(args, "POST", "/api/history/delete", Some(value))
                                        .await?,
                                    compact,
                                )?;
                            }
                        }
                        "retry" => {
                            if args.boolean("dry-run") {
                                cli::emit(&entry["commands"], compact)?;
                            } else {
                                let result = cli::request(
                                    args,
                                    "POST",
                                    &format!("/api/history/{index}/retry"),
                                    Some(json!({})),
                                )
                                .await?;
                                submitted(result, args).await?;
                            }
                        }
                        _ => return Err(cli::usage("unknown history action")),
                    }
                }
            }
        }
        "presets" => {
            let kind = args.value("kind");
            let downloads = kind == "download";
            let builtin = ["video", "convert"].contains(&kind.as_str());
            let mut cfg = cli::request(args, "GET", "/api/config", None).await?;
            let mut presets = if downloads {
                cfg["download_presets"].clone()
            } else if kind == "convert" {
                config::convert_presets()
            } else if kind == "video" {
                config::video_presets()
            } else {
                cli::request(args, "GET", "/api/pipeline/presets", None).await?["presets"].clone()
            };
            match action(args) {
                "list" => cli::emit(&presets, compact)?,
                other => {
                    let id = args.positionals.get(1).context("preset ID required")?;
                    match other {
                        "show" => cli::emit(
                            presets
                                .as_array()
                                .context("invalid presets")?
                                .iter()
                                .find(|p| p["id"] == *id)
                                .context("preset not found")?,
                            compact,
                        )?,
                        "delete" | "save" => {
                            ensure!(!builtin, "built-in presets are read-only");
                            let mut preset = json!({});
                            if other == "save" {
                                preset = storage::read_json(
                                    &config::user_path(&args.value("file")),
                                    Value::Null,
                                )?;
                                ensure!(preset.is_object(), "preset object required");
                                preset["id"] = json!(id);
                                if string(&preset, "name").is_empty() {
                                    preset["name"] = json!(id);
                                }
                                if !downloads {
                                    let operations: Vec<media::Operation> =
                                        serde_json::from_value(preset["operations"].clone())?;
                                    for operation in operations {
                                        media::validate_operation(&operation)?;
                                    }
                                }
                            }
                            if args.boolean("dry-run") {
                                cli::emit(&json!({"action":other,"id":id}), compact)?;
                            } else if downloads {
                                let entries = presets.as_array_mut().context("invalid presets")?;
                                entries.retain(|p| p["id"] != *id);
                                if other == "save" {
                                    entries.push(preset);
                                }
                                cfg["download_presets"] = presets;
                                cli::emit(
                                    &cli::request(args, "PUT", "/api/config", Some(cfg)).await?,
                                    compact,
                                )?;
                            } else if other == "save" {
                                cli::emit(
                                    &cli::request(
                                        args,
                                        "POST",
                                        "/api/pipeline/presets",
                                        Some(preset),
                                    )
                                    .await?,
                                    compact,
                                )?;
                            } else {
                                cli::emit(
                                    &cli::request(
                                        args,
                                        "DELETE",
                                        &format!("/api/pipeline/presets/{}", escaped(id)),
                                        None,
                                    )
                                    .await?,
                                    compact,
                                )?;
                            }
                        }
                        _ => return Err(cli::usage("unknown presets action")),
                    }
                }
            }
        }
        "convert" | "transcode" | "trim" | "aspect" | "mux" | "demux" | "concat" | "filter"
        | "audio" | "subtitle" | "metadata" | "scale" | "fps" | "stabilize" | "pipeline"
        | "editor" | "size" => {
            let pipeline = pipeline(command, args).await?;
            if args.has("save-project") {
                if !args.boolean("dry-run") {
                    storage::write_json(
                        &config::user_path(&args.value("save-project")),
                        &pipeline,
                    )?;
                }
                cli::emit(&json!({"saved":args.value("save-project")}), compact)?;
                return Ok(true);
            }
            let mut payload = serde_json::to_value(pipeline)?;
            let preview = args.number("preview", 0.0)?;
            let endpoint = if args.boolean("dry-run") {
                "/api/pipeline/dry-run"
            } else if preview > 0.0 {
                payload["duration"] = json!(preview);
                "/api/pipeline/preview"
            } else {
                "/api/pipeline/run"
            };
            let result = cli::request(args, "POST", endpoint, Some(payload)).await?;
            if args.boolean("dry-run") || preview > 0.0 {
                cli::emit(&result, compact)?;
            } else {
                submitted(result, args).await?;
            }
        }
        _ => return Ok(false),
    }
    Ok(true)
}
pub async fn pipeline(command: &str, args: &Arguments) -> Result<Pipeline> {
    let mut parsed = args.clone();
    let saved = if parsed.has("preset") && !["convert", "transcode"].contains(&command) {
        let id = parsed.value("preset");
        parsed.flags.remove("preset");
        let response = cli::request(args, "GET", "/api/pipeline/presets", None).await?;
        Some(
            response["presets"]
                .as_array()
                .context("invalid presets")?
                .iter()
                .find(|p| p["id"] == id)
                .context("preset not found")?["operations"]
                .clone(),
        )
    } else {
        None
    };
    let mut pipeline = cli::make_pipeline(command, &parsed)?;
    if let Some(operations) = saved {
        let mut saved: Vec<media::Operation> = serde_json::from_value(operations)?;
        saved.append(&mut pipeline.operations);
        pipeline.operations = saved;
    }
    Ok(pipeline)
}
pub async fn submitted(result: Value, args: &Arguments) -> Result<()> {
    let ids: Vec<u64> = if let Some(ids) = result["task_ids"].as_array() {
        ids.iter().filter_map(Value::as_u64).collect()
    } else {
        vec![result["task_id"].as_u64().context("missing task ID")?]
    };
    cli::emit(&result, args.boolean("json"))?;
    if !args.boolean("background") {
        cli::wait_remote(&ids, args).await?;
    }
    Ok(())
}
