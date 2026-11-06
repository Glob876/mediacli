use crate::{
    cli::{self, Arguments},
    config::{self, number, string},
    download,
    media::{self, Pipeline, Plan},
    queue::Queue,
    storage,
};
use anyhow::{Context, Result, bail, ensure};
use axum::{
    Router,
    body::{Body, Bytes},
    extract::{DefaultBodyLimit, State},
    http::{HeaderMap, Method, StatusCode, Uri, header},
    response::{
        IntoResponse, Response,
        sse::{Event, KeepAlive, Sse},
    },
};
use serde_json::{Value, json};
use std::{
    collections::HashMap,
    convert::Infallible,
    path::{Path, PathBuf},
    sync::Arc,
    time::Duration,
};
use tokio::io::{AsyncReadExt, AsyncSeekExt};
#[derive(Clone)]
pub struct Server {
    pub queue: Queue,
    pub token: String,
}
impl Server {
    pub fn router(self) -> Router {
        Router::new()
            .fallback(handle)
            .layer(DefaultBodyLimit::max(2 * 1024 * 1024))
            .with_state(Arc::new(self))
    }
}
pub async fn run(args: &Arguments) -> Result<()> {
    let cfg = config::load()?;
    let persistence = if args.boolean("persist") || args.has("queue-file") {
        Some(if args.has("queue-file") {
            config::user_path(&args.value("queue-file"))
        } else {
            config::directory().join("daemon_queue.json")
        })
    } else {
        None
    };
    let queue = Queue::new(number(&cfg, "bg_queue_max", 3.0) as usize, persistence)?;
    let token = if args.has("token") {
        args.value("token")
    } else {
        std::env::var("MEDIACLI_TOKEN").unwrap_or_default()
    };
    let port = args.number("port", 0.0)?;
    ensure!(
        port.fract() == 0.0 && (0.0..=65535.0).contains(&port),
        "invalid port"
    );
    let listener =
        tokio::net::TcpListener::bind((std::net::Ipv4Addr::LOCALHOST, port as u16)).await?;
    println!("MEDIACLI_DAEMON_PORT={}", listener.local_addr()?.port());
    let shutdown_queue = queue.clone();
    axum::serve(listener, Server { queue, token }.router())
        .with_graceful_shutdown(async move {
            shutdown_signal().await;
            shutdown_queue.shutdown().await;
        })
        .await?;
    Ok(())
}
async fn shutdown_signal() {
    #[cfg(unix)]
    {
        let mut terminate =
            tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
                .expect("signal handler");
        tokio::select! {_ = tokio::signal::ctrl_c()=>{},_ = terminate.recv()=>{}}
    }
    #[cfg(not(unix))]
    {
        let _ = tokio::signal::ctrl_c().await;
    }
}
fn query(uri: &Uri) -> HashMap<String, String> {
    let url = format!("http://localhost{uri}");
    reqwest::Url::parse(&url)
        .unwrap()
        .query_pairs()
        .into_owned()
        .collect()
}
fn error(code: StatusCode, message: impl ToString) -> Response {
    (code, axum::Json(json!({"error":message.to_string()}))).into_response()
}
async fn handle(
    State(server): State<Arc<Server>>,
    method: Method,
    uri: Uri,
    headers: HeaderMap,
    body: Bytes,
) -> Response {
    let response = handle_inner(server, method, uri, headers, body).await;
    let mut response = match response {
        Ok(response) => response,
        Err(error_value) => error(StatusCode::BAD_REQUEST, format!("{error_value:#}")),
    };
    response
        .headers_mut()
        .insert("access-control-allow-origin", "*".parse().unwrap());
    response.headers_mut().insert(
        "access-control-allow-headers",
        "Authorization, Content-Type, Range".parse().unwrap(),
    );
    response.headers_mut().insert(
        "access-control-allow-methods",
        "GET, POST, PUT, DELETE, OPTIONS".parse().unwrap(),
    );
    response
}
async fn handle_inner(
    server: Arc<Server>,
    method: Method,
    uri: Uri,
    headers: HeaderMap,
    body: Bytes,
) -> Result<Response> {
    if method == Method::OPTIONS {
        return Ok(StatusCode::NO_CONTENT.into_response());
    }
    let parameters = query(&uri);
    let path = uri.path().to_string();
    let authorization = headers
        .get(header::AUTHORIZATION)
        .and_then(|h| h.to_str().ok())
        .unwrap_or("");
    let bearer = authorization
        .split_once(' ')
        .filter(|(scheme, _)| scheme.eq_ignore_ascii_case("bearer"))
        .map(|(_, value)| value);
    if !server.token.is_empty()
        && bearer != Some(server.token.as_str())
        && parameters.get("token") != Some(&server.token)
    {
        return Ok(error(
            StatusCode::UNAUTHORIZED,
            "missing or invalid bearer token",
        ));
    }
    if path.starts_with("/api/tasks/") && path.ends_with("/events") && method == Method::GET {
        let id: u64 = path
            .trim_start_matches("/api/tasks/")
            .trim_end_matches("/events")
            .parse()?;
        ensure!(server.queue.get(id).is_some(), "task not found");
        let queue = server.queue.clone();
        let stream = async_stream::stream! {loop{let Some(task)=queue.get(id)else{break;};yield Ok::<Event,Infallible>(Event::default().json_data(task.snapshot()).unwrap());if task.finalized{break;}tokio::time::sleep(Duration::from_millis(250)).await;}};
        return Ok(Sse::new(stream)
            .keep_alive(KeepAlive::default())
            .into_response());
    }
    if ["/api/library/file", "/api/media/file", "/api/library/thumb"].contains(&path.as_str())
        && (method == Method::GET || method == Method::HEAD)
    {
        let cfg = config::load()?;
        let file = if path == "/api/media/file" {
            let file = config::user_path(parameters.get("path").context("path required")?);
            ensure!(file.is_file(), "file not found");
            file
        } else {
            library_path(parameters.get("name").context("name required")?, &cfg)?
        };
        let file = if path.ends_with("/thumb") {
            let source = media::text_path(&file);
            let cfg = cfg.clone();
            tokio::task::spawn_blocking(move || -> Result<PathBuf> {
                let directory = config::directory().join("thumbs");
                std::fs::create_dir_all(&directory)?;
                let target = directory.join(format!("{}.jpg", media::timestamp()));
                media::capture(
                    &vec![
                        config::tool(&cfg, "ffmpeg"),
                        "-nostdin".into(),
                        "-v".into(),
                        "error".into(),
                        "-y".into(),
                        "-i".into(),
                        source,
                        "-frames:v".into(),
                        "1".into(),
                        "-vf".into(),
                        "scale=320:-2".into(),
                        media::text_path(&target),
                    ],
                    Duration::from_secs(15),
                )?;
                Ok(target)
            })
            .await??
        } else {
            file
        };
        return file_response(file, &headers, method == Method::HEAD).await;
    }
    let value: Value = if body.is_empty() {
        json!({})
    } else {
        serde_json::from_slice(&body).context("invalid JSON body")?
    };
    let result =
        tokio::task::spawn_blocking(move || dispatch(&server, &method, &path, &parameters, value))
            .await??;
    Ok((StatusCode::from_u16(result.0)?, axum::Json(result.1)).into_response())
}
fn dispatch(
    server: &Server,
    method: &Method,
    path: &str,
    q: &HashMap<String, String>,
    value: Value,
) -> Result<(u16, Value)> {
    let cfg = config::load()?;
    let param = |key: &str| q.get(key).cloned().unwrap_or_default();
    match (method.as_str(), path) {
        ("GET", "/api/status") => Ok((
            200,
            json!({"ok":true,"version":crate::VERSION,"dependencies":cli::dependencies(&cfg),"queue_error":server.queue.persistence_error()}),
        )),
        ("GET", "/api/config") => Ok((200, cfg)),
        ("PUT", "/api/config") => {
            let mut updated = cfg;
            config::merge(&mut updated, &value)?;
            config::save(&updated)?;
            server
                .queue
                .set_maximum(number(&updated, "bg_queue_max", 3.0) as usize);
            Ok((200, updated))
        }
        ("POST", "/api/config/reset") => {
            let defaults = config::defaults();
            config::save(&defaults)?;
            server.queue.set_maximum(3);
            Ok((200, defaults))
        }
        ("GET", "/api/meta") => Ok((200, config::metadata())),
        ("GET", "/api/presets") => Ok((
            200,
            json!({"video_presets":config::video_presets(),"download_presets":cfg["download_presets"]}),
        )),
        ("GET", "/api/convert/presets") => {
            Ok((200, json!({"convert_presets":config::convert_presets()})))
        }
        ("GET", "/api/tasks") => Ok((
            200,
            json!({"tasks":server.queue.list().iter().map(|t|t.snapshot()).collect::<Vec<_>>()}),
        )),
        ("POST", "/api/jobs") => {
            let plan: Plan = serde_json::from_value(value["plan"].clone())?;
            let id = server.queue.enqueue(
                plan,
                &string(&value, "title"),
                &string(&value, "source"),
                "",
            )?;
            Ok((202, json!({"task_id":id})))
        }
        ("POST", "/api/downloads") => {
            let url = string(&value, "url");
            let fields = if value["fields"].is_null() {
                json!({})
            } else {
                value["fields"].clone()
            };
            let plan = download::build(
                &url,
                &fields,
                &cfg,
                &string(&value, "out_dir"),
                config::boolean(&value, "is_playlist"),
                None,
            )?;
            let id = server.queue.enqueue(plan, "Download", &url, "")?;
            Ok((202, json!({"task_id":id})))
        }
        ("POST", "/api/convert") => {
            let input = string(&value, "input");
            let mut pipeline = media::preset_pipeline(&string(&value, "preset_id"), &input)?;
            pipeline.output.path = string(&value, "output");
            let plan = media::build(&pipeline, &cfg)?;
            let target = plan.target.clone();
            let id = server.queue.enqueue(plan, "Convert", &input, "")?;
            Ok((202, json!({"task_id":id,"output":target})))
        }
        ("GET", "/api/history") => Ok((200, json!({"history":config::history()?}))),
        ("DELETE", "/api/history") => {
            storage::update_json(
                &config::directory().join("history.json"),
                json!([]),
                |history| {
                    *history = json!([]);
                    Ok(())
                },
            )?;
            Ok((200, json!({"ok":true})))
        }
        ("POST", "/api/history/delete") => {
            delete_history(&value, config::boolean(&value, "delete_file"), &cfg)?;
            Ok((200, json!({"ok":true})))
        }
        ("GET", "/api/library") => Ok((200, library(&cfg)?)),
        ("GET", "/api/browse") => {
            let path = param("path");
            Ok((
                200,
                browse(if path.is_empty() {
                    config::user_path(&string(&cfg, "download_dir"))
                } else {
                    config::user_path(&path)
                })?,
            ))
        }
        ("GET", "/api/probe") => {
            let path = param("path");
            let path = if path.is_empty() { param("name") } else { path };
            let input = if config::user_path(&path).is_file() {
                path
            } else {
                media::text_path(&library_path(&path, &cfg)?)
            };
            Ok((200, media::probe(&input, &cfg)?))
        }
        ("GET", "/api/tools/ffmpeg") | ("GET", "/api/tools/ffprobe") => {
            let name = path.rsplit('/').next().unwrap();
            let candidate = param("path");
            let mut cfg = cfg;
            if !candidate.is_empty() {
                cfg[format!("{name}_path")] = json!(candidate);
            }
            let binary = config::tool(&cfg, name);
            let result =
                media::capture(&[binary.clone(), "-version".into()], Duration::from_secs(5));
            let result = match result {
                Ok(bytes) => {
                    json!({"found":true,"resolved":which::which(&binary).map(|p|media::text_path(&p)).unwrap_or(binary.clone()),"configured":cfg[format!("{name}_path")],"version":String::from_utf8_lossy(&bytes).lines().next().unwrap_or("")})
                }
                Err(error) => {
                    json!({"found":false,"resolved":binary,"configured":cfg[format!("{name}_path")],"error":error.to_string()})
                }
            };
            Ok((200, result))
        }
        ("GET", "/api/pipeline/presets") => Ok((200, json!({"presets":config::media_presets()?}))),
        ("POST", "/api/pipeline/presets") => {
            let mut preset = value;
            ensure!(preset.is_object(), "preset object required");
            let operations: Vec<media::Operation> =
                serde_json::from_value(preset["operations"].clone())?;
            for operation in operations {
                media::validate_operation(&operation)?;
            }
            if string(&preset, "id").is_empty() {
                preset["id"] = json!(format!("mp_{}", media::timestamp()));
            }
            preset["created_at"] = json!(crate::queue::now());
            let saved = preset.clone();
            storage::update_json(
                &config::directory().join("media_presets.json"),
                json!([]),
                |presets| {
                    let entries = presets.as_array_mut().context("invalid presets")?;
                    entries.retain(|p| p["id"] != preset["id"]);
                    entries.push(preset);
                    Ok(())
                },
            )?;
            Ok((200, json!({"ok":true,"preset":saved})))
        }
        ("POST", "/api/pipeline/run")
        | ("POST", "/api/pipeline/dry-run")
        | ("POST", "/api/pipeline/preview") => {
            let mut pipeline: Pipeline = serde_json::from_value(value.clone())?;
            if path.ends_with("preview") {
                pipeline.output.path = media::text_path(
                    &std::env::temp_dir()
                        .join(format!("mediacli_preview_{}.mp4", media::timestamp())),
                );
                pipeline.output.target_megabytes = 0.0;
                pipeline.output.two_pass = false;
            }
            let mut plan = media::build(&pipeline, &cfg)?;
            if path.ends_with("dry-run") {
                let commands: Vec<_> = plan
                    .commands
                    .iter()
                    .map(|c| media::display_command(c))
                    .collect();
                return Ok((200, json!({"count":commands.len(),"commands":commands})));
            }
            if path.ends_with("preview") {
                let seconds = number(&value, "duration", 5.0);
                ensure!(
                    (0.1..=15.0).contains(&seconds),
                    "preview duration must be 0.1–15 seconds"
                );
                let command = plan.commands.last_mut().context("no preview command")?;
                let output = command.pop().unwrap();
                command.extend([
                    "-t".into(),
                    seconds.to_string(),
                    "-c:v".into(),
                    "libx264".into(),
                    "-c:a".into(),
                    "aac".into(),
                    "-pix_fmt".into(),
                    "yuv420p".into(),
                    "-movflags".into(),
                    "+faststart".into(),
                    output,
                ]);
                let displayed = media::display_command(command);
                for command in &plan.commands {
                    media::capture(command, Duration::from_secs(30))?;
                }
                return Ok((200, json!({"preview_path":plan.target,"command":displayed})));
            }
            submit(
                server,
                plan,
                &format!("Pipeline: {}", pipeline.name),
                &pipeline.inputs.join(","),
            )
        }
        ("POST", "/api/batch") => {
            let pipeline: Pipeline = serde_json::from_value(value["pipeline"].clone())?;
            let inputs: Vec<String> = serde_json::from_value(value["inputs"].clone())?;
            let (report, plans) = cli::batch_report(
                &inputs,
                &pipeline,
                &string(&value, "output_dir"),
                &string(&value, "output_template"),
                &cfg,
            )?;
            if config::boolean(&value, "dry_run") {
                return Ok((200, report));
            }
            ensure!(
                report["invalid_count"] == 0 || config::boolean(&value, "skip_invalid"),
                "batch validation failed"
            );
            let group = format!("batch_{}", media::timestamp());
            let mut ids = Vec::new();
            for (index, plan) in plans.into_iter().enumerate() {
                ids.push(server.queue.enqueue(
                    plan,
                    "Batch",
                    inputs.get(index).map(String::as_str).unwrap_or(""),
                    &group,
                )?);
            }
            Ok((
                202,
                json!({"task_ids":ids,"count":ids.len(),"report":report,"group_id":group}),
            ))
        }
        _ => {
            if let Some(rest) = path
                .strip_prefix("/api/tasks/group/")
                .and_then(|s| s.strip_suffix("/cancel"))
            {
                ensure!(method == Method::POST, "method not allowed");
                return Ok((
                    200,
                    json!({"cancelled":server.queue.cancel_group(rest),"group_id":rest}),
                ));
            }
            if let Some(rest) = path.strip_prefix("/api/tasks/") {
                let (raw, action) = rest.split_once('/').unwrap_or((rest, ""));
                let id = raw.parse::<u64>()?;
                let task = server.queue.get(id).context("task not found")?;
                return match (method.as_str(), action) {
                    ("GET", "") => Ok((200, task.snapshot())),
                    ("GET", "logs") => {
                        let after = param("after")
                            .parse::<usize>()
                            .unwrap_or(0)
                            .min(task.log_tail.len());
                        let end = (after + 2000).min(task.log_tail.len());
                        Ok((
                            200,
                            json!({"logs":task.log_tail[after..end],"next":end,"total":task.log_tail.len()}),
                        ))
                    }
                    ("POST", "cancel") => Ok((200, json!({"ok":server.queue.cancel(id),"id":id}))),
                    ("POST", "retry") => Ok((202, json!({"task_ids":[server.queue.retry(id)?]}))),
                    _ => bail!("unknown task endpoint"),
                };
            }
            if let Some(raw) = path
                .strip_prefix("/api/history/")
                .and_then(|s| s.strip_suffix("/retry"))
            {
                ensure!(method == Method::POST, "method not allowed");
                let index: usize = raw.parse()?;
                let history = config::history()?;
                let entry = history
                    .as_array()
                    .and_then(|a| a.get(index))
                    .context("history entry not found")?;
                let commands = serde_json::from_value(entry["commands"].clone())?;
                let id = server.queue.enqueue(
                    Plan {
                        commands,
                        target: string(entry, "target"),
                        replacement: None,
                    },
                    &string(entry, "type"),
                    &string(entry, "source"),
                    "",
                )?;
                return Ok((202, json!({"task_ids":[id]})));
            }
            if let Some(id) = path.strip_prefix("/api/pipeline/presets/") {
                ensure!(method == Method::DELETE, "method not allowed");
                storage::update_json(
                    &config::directory().join("media_presets.json"),
                    json!([]),
                    |presets| {
                        presets
                            .as_array_mut()
                            .context("invalid presets")?
                            .retain(|p| p["id"] != id);
                        Ok(())
                    },
                )?;
                return Ok((200, json!({"ok":true})));
            }
            if let Some(kind) = path.strip_prefix("/api/operations/") {
                ensure!(method == Method::POST, "method not allowed");
                let inputs = if value["inputs"].is_array() {
                    serde_json::from_value(value["inputs"].clone())?
                } else {
                    vec![string(&value, "input")]
                };
                let operation = value.get(kind).cloned().unwrap_or(json!({}));
                let mut pipeline = Pipeline {
                    inputs,
                    operations: vec![media::operation(kind, operation)],
                    ..Default::default()
                };
                if kind == "concat" {
                    pipeline.operations[0].config["inputs"] = json!(pipeline.inputs);
                }
                if kind == "mux" && pipeline.inputs.len() > 1 {
                    if pipeline.operations[0].config["extra_inputs"].is_null() {
                        pipeline.operations[0].config["extra_inputs"] =
                            json!(&pipeline.inputs[1..]);
                    }
                    pipeline.inputs.truncate(1);
                }
                pipeline.output.path = string(&value, "output");
                let plan = media::build(&pipeline, &cfg)?;
                return submit(server, plan, kind, &pipeline.inputs.join(","));
            }
            Ok((404, json!({"error":"endpoint not found"})))
        }
    }
}
fn submit(server: &Server, plan: Plan, title: &str, source: &str) -> Result<(u16, Value)> {
    let group = format!("pipeline_{}", media::timestamp());
    let target = plan.target.clone();
    let commands = plan.commands.clone();
    let id = server.queue.enqueue(plan, title, source, &group)?;
    Ok((
        202,
        json!({"task_id":id,"task_ids":[id],"group_id":group,"target":target,"output":target,"commands":commands}),
    ))
}
pub fn library_path(name: &str, cfg: &Value) -> Result<PathBuf> {
    let root = std::fs::canonicalize(config::user_path(&string(cfg, "download_dir")))?;
    let path = root.join(name);
    let resolved = std::fs::canonicalize(path)?;
    ensure!(
        resolved.starts_with(&root) && resolved.is_file(),
        "file is outside download directory"
    );
    Ok(resolved)
}
pub fn delete_history(entry: &Value, delete_file: bool, cfg: &Value) -> Result<()> {
    if delete_file {
        let target = string(entry, "target");
        if !target.is_empty() {
            if let Ok(path) = library_path(&target, cfg) {
                std::fs::remove_file(path)?;
            }
        }
    }
    storage::update_json(
        &config::directory().join("history.json"),
        json!([]),
        |history| {
            history
                .as_array_mut()
                .context("invalid history")?
                .retain(|e| {
                    !(e["time"] == entry["time"]
                        && e["source"] == entry["source"]
                        && e["target"] == entry["target"])
                });
            Ok(())
        },
    )?;
    Ok(())
}
fn is_media(path: &Path) -> bool {
    let ext = path
        .extension()
        .unwrap_or_default()
        .to_string_lossy()
        .to_lowercase();
    [
        "mp4", "mkv", "mov", "webm", "avi", "mp3", "m4a", "flac", "wav", "opus", "ogg", "m4v",
        "gif", "aac",
    ]
    .contains(&ext.as_str())
}
pub fn browse(directory: PathBuf) -> Result<Value> {
    let directory = std::path::absolute(directory)?;
    let mut dirs = Vec::new();
    let mut files = Vec::new();
    for entry in std::fs::read_dir(&directory)? {
        let entry = entry?;
        let path = entry.path();
        let name = entry.file_name().to_string_lossy().into_owned();
        let info = entry.metadata()?;
        if info.is_dir() {
            dirs.push(name);
            continue;
        }
        if !info.is_file()
            || name.ends_with(".part")
            || name.ends_with(".ytdl")
            || name.ends_with(".tmp")
            || name.contains(".temp.")
        {
            continue;
        }
        let modified: chrono::DateTime<chrono::Local> = info.modified()?.into();
        files.push(json!({"name":name,"size":info.len(),"mtime":modified.format("%Y-%m-%d %H:%M:%S").to_string(),"is_media":is_media(&path)}));
    }
    dirs.sort();
    files.sort_by_key(|f| string(f, "name"));
    Ok(
        json!({"path":directory,"parent":directory.parent().map(media::text_path).unwrap_or_default(),"dirs":dirs,"files":files}),
    )
}
pub fn library(cfg: &Value) -> Result<Value> {
    let directory = config::user_path(&string(cfg, "download_dir"));
    if !directory.exists() {
        return Ok(json!({"dir":directory,"files":[]}));
    }
    let result = browse(directory.clone())?;
    Ok(json!({"dir":directory,"files":result["files"]}))
}
async fn file_response(path: PathBuf, headers: &HeaderMap, head: bool) -> Result<Response> {
    let mut file = tokio::fs::File::open(&path).await?;
    let length = file.metadata().await?.len();
    let mut start = 0;
    let mut end = length.saturating_sub(1);
    let mut partial = false;
    if let Some(raw) = headers.get(header::RANGE) {
        let range = raw
            .to_str()?
            .strip_prefix("bytes=")
            .context("unsupported range")?;
        if range.contains(',') {
            return Ok((
                StatusCode::RANGE_NOT_SATISFIABLE,
                [(header::CONTENT_RANGE, format!("bytes */{length}"))],
            )
                .into_response());
        }
        let (from, to) = range.split_once('-').context("invalid byte range")?;
        if from.is_empty() {
            let suffix = to.parse::<u64>()?;
            ensure!(suffix > 0, "invalid suffix");
            start = length.saturating_sub(suffix);
        } else {
            start = from.parse()?;
            if !to.is_empty() {
                end = to.parse::<u64>()?.min(end);
            }
        }
        if length == 0 || start >= length || end < start {
            return Ok((
                StatusCode::RANGE_NOT_SATISFIABLE,
                [(header::CONTENT_RANGE, format!("bytes */{length}"))],
            )
                .into_response());
        }
        partial = true;
    }
    let size = if length == 0 { 0 } else { end - start + 1 };
    file.seek(std::io::SeekFrom::Start(start)).await?;
    let extension = path
        .extension()
        .unwrap_or_default()
        .to_string_lossy()
        .to_lowercase();
    let mime = match extension.as_str() {
        "mp4" | "m4v" => "video/mp4",
        "webm" => "video/webm",
        "mkv" => "video/x-matroska",
        "mov" => "video/quicktime",
        "mp3" => "audio/mpeg",
        "m4a" => "audio/mp4",
        "wav" => "audio/wav",
        "flac" => "audio/flac",
        "png" => "image/png",
        "jpg" | "jpeg" => "image/jpeg",
        "webp" => "image/webp",
        _ => "application/octet-stream",
    };
    let body = if head {
        Body::empty()
    } else {
        Body::from_stream(tokio_util::io::ReaderStream::new(file.take(size)))
    };
    let mut response = Response::builder()
        .status(if partial { 206 } else { 200 })
        .header(header::CONTENT_TYPE, mime)
        .header(header::CONTENT_LENGTH, size)
        .header(header::ACCEPT_RANGES, "bytes");
    if partial {
        response = response.header(
            header::CONTENT_RANGE,
            format!("bytes {start}-{end}/{length}"),
        );
    }
    Ok(response.body(body)?)
}
