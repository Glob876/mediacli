use crate::{
    config::{self, boolean, number, string},
    download,
    media::{self, Pipeline, Plan},
    queue::Queue,
    storage,
};
use anyhow::{Context, Result, ensure};
use serde_json::{Value, json};
use std::{
    collections::{BTreeMap, BTreeSet},
    fmt,
    io::{self, Read},
    path::Path,
    time::Duration,
};

#[derive(Debug)]
pub struct Exit(pub i32);
impl fmt::Display for Exit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "exit status {}", self.0)
    }
}
impl std::error::Error for Exit {}
pub fn usage(message: impl Into<String>) -> anyhow::Error {
    anyhow::Error::new(Exit(2)).context(message.into())
}
#[derive(Clone, Debug, Default)]
pub struct Arguments {
    pub positionals: Vec<String>,
    pub flags: BTreeMap<String, Vec<String>>,
}
impl Arguments {
    pub fn value(&self, key: &str) -> String {
        self.flags
            .get(key)
            .and_then(|v| v.last())
            .cloned()
            .unwrap_or_default()
    }
    pub fn all(&self, key: &str) -> Vec<String> {
        self.flags.get(key).cloned().unwrap_or_default()
    }
    pub fn has(&self, key: &str) -> bool {
        self.flags.contains_key(key)
    }
    pub fn boolean(&self, key: &str) -> bool {
        self.has(key) && self.value(key) != "false"
    }
    pub fn number(&self, key: &str, default: f64) -> Result<f64> {
        if self.has(key) {
            let number = self
                .value(key)
                .parse::<f64>()
                .map_err(|_| usage(format!("invalid --{key}")))?;
            ensure!(number.is_finite(), "invalid --{key}");
            Ok(number)
        } else {
            Ok(default)
        }
    }
    pub fn parse(args: &[String], command: &str) -> Result<Self> {
        let schema = config::schema();
        let kind = if command == "convert" || command == "transcode" {
            "transcode"
        } else {
            command
        };
        let mut known:BTreeSet<String>="json dry-run background server token timeout input output o project preset save-project options overwrite ratio size-mb two-pass preview join directory output-dir output-template skip-invalid op range quality playlist audio-only audio-format fields set format transcode cookies-mode cookies-browser cookies-file subs-enabled sub-langs embed-subs auto-subs embed-metadata embed-thumbnail embed-chapters video-preset fps-limit vcodec proxy-mode proxy-url concurrent-fragments thumbnail-format force-overwrite search kind file yes y delete-file start end columns port persist queue-file name".split_whitespace().map(str::to_string).collect();
        let mut booleans:BTreeSet<String>="json dry-run background overwrite two-pass join skip-invalid playlist audio-only subs-enabled embed-subs auto-subs embed-metadata embed-thumbnail embed-chapters force-overwrite yes y delete-file persist".split_whitespace().map(str::to_string).collect();
        if let Some(fields) = schema[kind].as_object() {
            for (key, ty) in fields {
                let flag = field_flag(key);
                known.insert(flag.clone());
                if ty == "bool" {
                    booleans.insert(flag);
                }
            }
        }
        let mut parsed = Self::default();
        let mut index = 0;
        while index < args.len() {
            let argument = &args[index];
            if argument == "--" {
                parsed.positionals.extend_from_slice(&args[index + 1..]);
                break;
            }
            if !argument.starts_with('-') || argument == "-" {
                parsed.positionals.push(argument.clone());
                index += 1;
                continue;
            }
            let stripped = argument.trim_start_matches('-');
            let (key, inline) = stripped
                .split_once('=')
                .map(|(a, b)| (a, Some(b)))
                .unwrap_or((stripped, None));
            if !known.contains(key) {
                return Err(usage(format!("unknown flag: --{key}")));
            }
            let value = if let Some(value) = inline {
                value.to_owned()
            } else if booleans.contains(key) {
                "true".into()
            } else {
                index += 1;
                args.get(index)
                    .cloned()
                    .ok_or_else(|| usage(format!("missing value for --{key}")))?
            };
            parsed.flags.entry(key.into()).or_default().push(value);
            index += 1;
        }
        Ok(parsed)
    }
}
pub fn field_flag(key: &str) -> String {
    match key {
        "input" => "subtitle-file".into(),
        "preset" => "op-preset".into(),
        other => other.replace('_', "-"),
    }
}
pub fn read_value(raw: &str) -> Result<Value> {
    if let Some(path) = raw.strip_prefix('@') {
        return Ok(serde_json::from_slice(&std::fs::read(config::user_path(
            path,
        ))?)?);
    }
    Ok(serde_json::from_str(raw)?)
}
pub fn read_project(path: &str) -> Result<Pipeline> {
    let bytes = if path == "-" {
        let mut bytes = Vec::new();
        io::stdin().read_to_end(&mut bytes)?;
        bytes
    } else {
        let file = if ["editor", "pipeline"].contains(&path) {
            config::directory().join(format!("{path}_draft.json"))
        } else {
            config::user_path(path)
        };
        std::fs::read(file)?
    };
    Ok(serde_json::from_slice(&bytes)?)
}
pub fn emit(value: &Value, compact: bool) -> Result<()> {
    if compact {
        println!("{}", serde_json::to_string(value)?);
    } else if let Some(text) = value.as_str() {
        println!("{text}");
    } else {
        println!("{}", serde_json::to_string_pretty(value)?);
    }
    Ok(())
}

pub fn run(arguments: Vec<String>) -> Result<()> {
    if arguments
        .first()
        .is_some_and(|s| ["--version", "-v", "version"].contains(&s.as_str()))
    {
        println!("{}", crate::VERSION);
        return Ok(());
    }
    let runtime = tokio::runtime::Runtime::new()?;
    runtime.block_on(async {
        if arguments.is_empty() {
            return crate::tui::run().await;
        }
        let command = &arguments[0];
        if arguments
            .iter()
            .any(|s| ["--help", "-h"].contains(&s.as_str()))
            || command == "help"
        {
            help(command);
            return Ok(());
        }
        let args = Arguments::parse(&arguments[1..], command)?;
        if command == "daemon" {
            return crate::daemon::run(&args).await;
        }
        if ["gui", "--gui", "-gui", "-g"].contains(&command.as_str()) {
            println!("./build.sh --startelectron");
            return Ok(());
        }
        execute(command, &args).await
    })
}
pub fn help(command: &str) {
    println!(
        "MediaCLI {} (Rust)\nUsage: mediacli [command] [flags] [arguments]\n\nWithout arguments: interactive terminal interface.\nCommands:\n  download video thumbnail probe inspect convert transcode trim aspect\n  mux demux concat filter audio subtitle metadata scale fps stabilize\n  pipeline editor size ranges waveform batch config presets history\n  queue doctor cookies library daemon gui\n\nCommon flags: --json --dry-run --timeout 10m --server URL --token TOKEN\nMedia flags: --input FILE -o FILE --overwrite --project FILE\n  --op TYPE=JSON --options JSON --preset ID --save-project FILE\n  --size-mb MB --two-pass --preview SECONDS\nDownload flags: --quality HEIGHT --playlist --audio-only --audio-format FORMAT\n  --fields JSON/@FILE --set KEY=VALUE --output-dir DIRECTORY\n\nSee docs/cli.md for all commands and examples.",
        crate::VERSION
    );
    let schema = config::schema();
    let kind = if command == "convert" {
        "transcode"
    } else {
        command
    };
    if let Some(fields) = schema[kind].as_object() {
        println!("\n{command} fields:");
        for (key, ty) in fields {
            println!(
                "  --{} ({})",
                field_flag(key),
                ty.as_str().unwrap_or("JSON")
            );
        }
    }
}
pub fn duration(raw: &str) -> Result<Duration> {
    if raw.is_empty() || raw == "0" {
        return Ok(Duration::ZERO);
    }
    let (number, multiplier) = if let Some(v) = raw.strip_suffix("ms") {
        (v, 0.001)
    } else if let Some(v) = raw.strip_suffix('s') {
        (v, 1.0)
    } else if let Some(v) = raw.strip_suffix('m') {
        (v, 60.0)
    } else if let Some(v) = raw.strip_suffix('h') {
        (v, 3600.0)
    } else {
        (raw, 1.0)
    };
    let seconds = number
        .parse::<f64>()
        .map_err(|_| usage("invalid timeout"))?
        * multiplier;
    ensure!(
        seconds.is_finite() && (0.0..=31536000.0).contains(&seconds),
        "invalid timeout"
    );
    Ok(Duration::from_secs_f64(seconds))
}

pub async fn execute(command: &str, args: &Arguments) -> Result<()> {
    let cfg = config::load()?;
    match command {
        "config" => admin_config(args, &cfg),
        "presets" => admin_presets(args, &cfg),
        "history" => admin_history(args, &cfg).await,
        "doctor" => emit(&dependencies(&cfg), args.boolean("json")),
        "cookies" => {
            let mode = string(&cfg, "cookies_mode");
            emit(
                &json!({"mode":mode,"file":cfg["cookies_file"],"browser":cfg["cookies_browser"],"exists":config::user_path(&string(&cfg,"cookies_file")).is_file()}),
                args.boolean("json"),
            )
        }
        "library" => emit(&crate::daemon::library(&cfg)?, args.boolean("json")),
        "probe" | "inspect" => {
            let input = args
                .positionals
                .first()
                .cloned()
                .unwrap_or(args.value("input"));
            ensure!(!input.is_empty(), "input required");
            emit(&media::probe(&input, &cfg)?, args.boolean("json"))
        }
        "waveform" => {
            let input = args.positionals.first().context("input required")?;
            let peaks = media::waveform(
                input,
                args.number("start", 0.0)?,
                args.number("columns", 80.0)? as usize,
                &cfg,
            )?;
            if args.boolean("json") {
                emit(&json!(peaks), true)
            } else {
                let levels: Vec<char> = " ▁▂▃▄▅▆▇█".chars().collect();
                println!(
                    "{}",
                    peaks
                        .iter()
                        .map(|p| levels[(p * 8.0).round() as usize])
                        .collect::<String>()
                );
                Ok(())
            }
        }
        "queue" => remote_queue(args).await,
        "download" | "video" | "thumbnail" => {
            ensure!(!args.positionals.is_empty(), "URL required");
            let fields = download_options(args, &cfg)?;
            let mut plans = Vec::new();
            for url in &args.positionals {
                let format = args.value("format");
                let thumbnail = if command == "thumbnail" {
                    Some(if format.is_empty() { "png" } else { &format })
                } else {
                    None
                };
                plans.push((
                    download::build(
                        url,
                        &fields,
                        &cfg,
                        &args.value("output-dir"),
                        args.boolean("playlist"),
                        thumbnail,
                    )?,
                    command.into(),
                    url.clone(),
                ));
            }
            run_plans(plans, args, &cfg).await
        }
        "batch" => batch(args, &cfg).await,
        "ranges" => ranges(args, &cfg).await,
        "convert" | "transcode" | "trim" | "aspect" | "mux" | "demux" | "concat" | "filter"
        | "audio" | "subtitle" | "metadata" | "scale" | "fps" | "stabilize" | "pipeline"
        | "editor" | "size" => {
            let pipeline = make_pipeline(command, args)?;
            if args.has("save-project") {
                storage::write_json(&config::user_path(&args.value("save-project")), &pipeline)?;
                return emit(
                    &json!({"saved":args.value("save-project")}),
                    args.boolean("json"),
                );
            }
            let mut plan = media::build(&pipeline, &cfg)?;
            let preview = args.number("preview", 0.0)?;
            if preview > 0.0 {
                ensure!(
                    (0.1..=15.0).contains(&preview),
                    "preview must be 0.1–15 seconds"
                );
                let last = plan.commands.last_mut().unwrap();
                let output = last.pop().unwrap();
                last.extend(["-t".into(), preview.to_string(), output]);
            }
            run_plans(
                vec![(plan, command.into(), pipeline.inputs.join(","))],
                args,
                &cfg,
            )
            .await
        }
        _ => Err(usage(format!("unknown command: {command}"))),
    }
}

pub fn download_options(args: &Arguments, cfg: &Value) -> Result<Value> {
    let mut fields = cfg["preset_defaults"].clone();
    if !fields.is_object() {
        fields = config::download_fields();
    }
    if args.has("preset") {
        let id = args.value("preset");
        if let Some(saved) = cfg["download_presets"]
            .as_array()
            .and_then(|a| a.iter().find(|p| p["id"] == id))
        {
            config::merge(&mut fields, &saved["fields"])?;
        } else {
            fields["video_preset"] = json!(id);
        }
    }
    if args.has("fields") {
        config::merge(&mut fields, &read_value(&args.value("fields"))?)?;
    }
    let known = config::download_fields();
    for (key, _) in known.as_object().unwrap() {
        let flag = key.replace('_', "-");
        if args.has(&flag) {
            let raw = args.value(&flag);
            fields[key] = if known[key].is_boolean() {
                json!(raw != "false")
            } else {
                json!(raw)
            };
        }
    }
    for setting in args.all("set") {
        let (key, value) = setting
            .split_once('=')
            .ok_or_else(|| usage("--set expects KEY=VALUE"))?;
        ensure!(
            known.get(key).is_some() || ["custom_ext", "custom_flags"].contains(&key),
            "unknown download field: {key}"
        );
        fields[key] = serde_json::from_str(value).unwrap_or(json!(value));
    }
    Ok(fields)
}

pub fn make_pipeline(command: &str, args: &Arguments) -> Result<Pipeline> {
    let mut pipeline = if args.has("project") {
        read_project(&args.value("project"))?
    } else {
        Pipeline::default()
    };
    if args.has("preset") && command != "convert" && command != "transcode" {
        let presets = config::media_presets()?;
        let id = args.value("preset");
        let saved = presets
            .as_array()
            .unwrap()
            .iter()
            .find(|p| p["id"] == id)
            .context("media preset not found")?;
        pipeline.operations = serde_json::from_value(saved["operations"].clone())?;
    }
    let mut inputs = args.all("input");
    inputs.extend(args.positionals.clone());
    if !inputs.is_empty() {
        pipeline.inputs = inputs;
    }
    ensure!(!pipeline.inputs.is_empty(), "input required");
    let kind = match command {
        "convert" => "transcode",
        "pipeline" | "editor" | "size" | "batch" => "",
        other => other,
    };
    if !kind.is_empty() {
        if kind == "transcode" && args.has("preset") {
            let built = media::preset_pipeline(&args.value("preset"), &pipeline.inputs[0])?;
            pipeline.operations.extend(built.operations);
        } else {
            let mut options = match kind {
                "transcode" => {
                    json!({"vcodec":"libx264","acodec":"aac","crf":"20","container":"mp4","preset":"medium"})
                }
                "trim" => json!({"start":"0","mode":"reencode"}),
                "aspect" => json!({"ratio_w":9,"ratio_h":16,"mode":"crop","size_preset":"1080p"}),
                "concat" => json!({"mode":"filter","inputs":pipeline.inputs}),
                "subtitle" => json!({"mode":"burn"}),
                "scale" => json!({"width":1280,"algo":"lanczos"}),
                "fps" => json!({"fps":"30"}),
                "demux" => json!({"stream_type":"audio"}),
                _ => json!({}),
            };
            if args.has("options") {
                config::merge(&mut options, &read_value(&args.value("options"))?)?;
            }
            let schema = config::schema();
            for (key, ty) in schema[kind].as_object().context("unknown operation")? {
                let flag = field_flag(key);
                if args.has(&flag) {
                    let raw = args.value(&flag);
                    options[key] = match ty.as_str().unwrap() {
                        "string" => json!(raw),
                        "[]string" => {
                            let mut values = Vec::new();
                            for raw in args.all(&flag) {
                                if raw.starts_with('[') {
                                    values.extend(serde_json::from_str::<Vec<String>>(&raw)?);
                                } else {
                                    values.push(raw);
                                }
                            }
                            json!(values)
                        }
                        _ => serde_json::from_str(&raw)
                            .with_context(|| format!("invalid --{flag}"))?,
                    };
                }
            }
            if kind == "aspect" && args.has("ratio") {
                let ratio = args.value("ratio");
                let (width, height) = ratio.split_once(':').context("ratio must be W:H")?;
                options["ratio_w"] = json!(width.parse::<i64>()?);
                options["ratio_h"] = json!(height.parse::<i64>()?);
            }
            pipeline.operations.push(media::operation(kind, options));
        }
    }
    for raw in args.all("op") {
        let (kind, body) = raw
            .split_once('=')
            .ok_or_else(|| usage("--op expects TYPE=JSON"))?;
        pipeline
            .operations
            .push(media::operation(kind, read_value(body)?));
    }
    let output = if args.has("o") {
        args.value("o")
    } else {
        args.value("output")
    };
    if !output.is_empty() {
        pipeline.output.path = output;
    }
    if args.has("overwrite") {
        pipeline.output.overwrite = args.boolean("overwrite");
    }
    if args.has("size-mb") {
        pipeline.output.target_megabytes = args.number("size-mb", 0.0)?;
        ensure!(
            pipeline.output.target_megabytes > 0.0,
            "size must be positive"
        );
        pipeline.output.two_pass = !args.has("two-pass") || args.boolean("two-pass");
    }
    if command == "size" {
        ensure!(pipeline.output.target_megabytes > 0.0, "--size-mb required");
    }
    Ok(pipeline)
}

pub async fn run_plans(
    plans: Vec<(Plan, String, String)>,
    args: &Arguments,
    cfg: &Value,
) -> Result<()> {
    if args.boolean("dry-run") {
        for (plan, _, _) in plans {
            emit(
                &json!({"commands":plan.commands.iter().map(|c|media::masked(c)).collect::<Vec<_>>(),"target":plan.target}),
                args.boolean("json"),
            )?;
        }
        return Ok(());
    }
    if !server(args).is_empty() {
        let mut ids = Vec::new();
        for (plan, title, source) in plans {
            let response = request(
                args,
                "POST",
                "/api/jobs",
                Some(json!({"plan":plan,"title":title,"source":source})),
            )
            .await?;
            ids.push(response["task_id"].as_u64().context("missing task ID")?);
        }
        emit(&json!({"task_ids":ids}), args.boolean("json"))?;
        if args.boolean("background") {
            return Ok(());
        }
        return wait_remote(&ids, args).await;
    }
    ensure!(
        !args.boolean("background"),
        "--background requires --server"
    );
    let queue = Queue::new(number(cfg, "bg_queue_max", 3.0) as usize, None)?;
    let mut ids = Vec::new();
    for (plan, title, source) in plans {
        ids.push(queue.enqueue(plan, &title, &source, "")?);
    }
    wait_local(&queue, &ids, args).await
}
pub async fn wait_local(queue: &Queue, ids: &[u64], args: &Arguments) -> Result<()> {
    let timeout = duration(&args.value("timeout"))?;
    let deadline = if timeout.is_zero() {
        None
    } else {
        Some(tokio::time::Instant::now() + timeout)
    };
    let mut offsets = BTreeMap::new();
    let mut interrupted = Box::pin(tokio::signal::ctrl_c());
    let mut done = BTreeSet::new();
    loop {
        for id in ids {
            let task = queue.get(*id).context("task not found")?;
            let offset = offsets.entry(*id).or_insert(0);
            for line in task.log_tail.iter().skip(*offset) {
                eprintln!("{line}");
            }
            *offset = task.log_tail.len();
            if task.finalized && done.insert(*id) {
                emit(&task.snapshot(), args.boolean("json"))?;
            }
        }
        if done.len() == ids.len() {
            let failed = ids
                .iter()
                .filter_map(|id| queue.get(*id))
                .find(|t| t.exit_code != 0);
            return if let Some(task) = failed {
                Err(
                    anyhow::Error::new(Exit(if task.exit_code == 130 { 130 } else { 1 }))
                        .context(format!("{} {}", task.title, task.status)),
                )
            } else {
                Ok(())
            };
        }
        tokio::select! {_ = &mut interrupted=>{queue.shutdown().await;return Err(Exit(130).into());},_ = tokio::time::sleep(Duration::from_millis(80))=>{}}
        if deadline.is_some_and(|d| tokio::time::Instant::now() >= d) {
            queue.shutdown().await;
            return Err(Exit(124).into());
        }
    }
}
pub fn server(args: &Arguments) -> String {
    let value = args.value("server");
    if value.is_empty() {
        std::env::var("MEDIACLI_SERVER").unwrap_or_default()
    } else {
        value
    }
}
pub async fn request(
    args: &Arguments,
    method: &str,
    path: &str,
    body: Option<Value>,
) -> Result<Value> {
    let base = server(args);
    ensure!(
        base.starts_with("http://127.0.0.1:")
            || base.starts_with("http://localhost:")
            || base.starts_with("https://"),
        "--server must be a loopback HTTP or HTTPS URL"
    );
    let token = if args.has("token") {
        args.value("token")
    } else {
        std::env::var("MEDIACLI_TOKEN").unwrap_or_default()
    };
    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(30))
        .build()?;
    let mut request = client
        .request(
            method.parse()?,
            format!("{}{path}", base.trim_end_matches('/')),
        )
        .bearer_auth(token);
    if let Some(value) = body {
        request = request.json(&value);
    }
    let response = request.send().await?;
    let status = response.status();
    let value: Value = response.json().await?;
    ensure!(
        status.is_success(),
        "daemon: {}",
        value["error"].as_str().unwrap_or("request failed")
    );
    Ok(value)
}
pub async fn wait_remote(ids: &[u64], args: &Arguments) -> Result<()> {
    let start = std::time::Instant::now();
    let timeout = duration(&args.value("timeout"))?;
    loop {
        let mut complete = true;
        let mut failed = false;
        for id in ids {
            let task = request(args, "GET", &format!("/api/tasks/{id}"), None).await?;
            complete &= boolean(&task, "finalized");
            failed |= boolean(&task, "finalized") && task["exit_code"] != 0;
        }
        if complete {
            ensure!(!failed, "remote task failed");
            return Ok(());
        }
        tokio::select! {_ = tokio::signal::ctrl_c()=>{for id in ids{let _=request(args,"POST",&format!("/api/tasks/{id}/cancel"),Some(json!({}))).await;}return Err(Exit(130).into());},_ = tokio::time::sleep(Duration::from_millis(200))=>{}}
        if !timeout.is_zero() && start.elapsed() > timeout {
            return Err(Exit(124).into());
        }
    }
}
pub async fn remote_queue(args: &Arguments) -> Result<()> {
    let action = args
        .positionals
        .first()
        .map(String::as_str)
        .unwrap_or("list");
    let id = args.positionals.get(1).cloned().unwrap_or_default();
    match action {
        "list" => emit(
            &request(args, "GET", "/api/tasks", None).await?,
            args.boolean("json"),
        ),
        "show" | "logs" => emit(
            &request(
                args,
                "GET",
                &format!(
                    "/api/tasks/{id}{}",
                    if action == "logs" { "/logs" } else { "" }
                ),
                None,
            )
            .await?,
            args.boolean("json"),
        ),
        "cancel" | "retry" => emit(
            &request(
                args,
                "POST",
                &format!("/api/tasks/{id}/{action}"),
                Some(json!({})),
            )
            .await?,
            args.boolean("json"),
        ),
        "wait" => wait_remote(&[id.parse()?], args).await,
        _ => Err(usage("unknown queue action")),
    }
}

pub fn dependencies(cfg: &Value) -> Value {
    json!(
        ["yt-dlp", "ffmpeg", "ffprobe"]
            .iter()
            .map(|name| {
                let binary = config::tool(cfg, name);
                let path = which::which(&binary).ok();
                json!({"name":name,"available":path.is_some(),"path":path,"required":true})
            })
            .collect::<Vec<_>>()
    )
}
fn admin_config(args: &Arguments, cfg: &Value) -> Result<()> {
    let action = args
        .positionals
        .first()
        .map(String::as_str)
        .unwrap_or("get");
    match action {
        "get" => {
            let value = if let Some(key) = args.positionals.get(1) {
                cfg.get(key).context("unknown setting")?
            } else {
                cfg
            };
            emit(value, args.boolean("json"))
        }
        "options" => emit(&config::metadata(), args.boolean("json")),
        "set" => {
            let key = args.positionals.get(1).context("setting required")?;
            let raw = args.positionals.get(2).context("value required")?;
            let default = cfg.get(key).context("unknown setting")?;
            let mut updated = cfg.clone();
            updated[key] = if default.is_string() {
                json!(raw)
            } else {
                serde_json::from_str(raw)?
            };
            config::save(&updated)?;
            emit(&updated, args.boolean("json"))
        }
        "reset" => {
            let defaults = config::defaults();
            config::save(&defaults)?;
            emit(&defaults, args.boolean("json"))
        }
        "export" => {
            let path = args.positionals.get(1).context("file required")?;
            storage::write_json(&config::user_path(path), cfg)
        }
        "import" => {
            let path = args.positionals.get(1).context("file required")?;
            let mut updated = config::defaults();
            let imported = storage::read_json(&config::user_path(path), Value::Null)?;
            config::merge(&mut updated, &imported)?;
            config::save(&updated)?;
            emit(&updated, args.boolean("json"))
        }
        _ => Err(usage("unknown config action")),
    }
}
fn admin_presets(args: &Arguments, cfg: &Value) -> Result<()> {
    let action = args
        .positionals
        .first()
        .map(String::as_str)
        .unwrap_or("list");
    let kind = args.value("kind");
    if action == "list" {
        return emit(
            &match kind.as_str() {
                "convert" => config::convert_presets(),
                "video" => config::video_presets(),
                "download" => cfg["download_presets"].clone(),
                _ => config::media_presets()?,
            },
            args.boolean("json"),
        );
    }
    let id = args.positionals.get(1).context("preset ID required")?;
    let is_download = kind == "download";
    let mut presets = if is_download {
        cfg["download_presets"].clone()
    } else {
        config::media_presets()?
    };
    match action {
        "show" => {
            return emit(
                presets
                    .as_array()
                    .unwrap()
                    .iter()
                    .find(|p| p["id"] == *id)
                    .context("preset not found")?,
                args.boolean("json"),
            );
        }
        "save" => {
            let path = args.value("file");
            ensure!(!path.is_empty(), "--file required");
            let mut preset = storage::read_json(&config::user_path(&path), Value::Null)?;
            ensure!(preset.is_object(), "preset must be an object");
            preset["id"] = json!(id);
            if !preset["name"].is_string() {
                preset["name"] = json!(id);
            }
            if !is_download {
                let operations: Vec<media::Operation> =
                    serde_json::from_value(preset["operations"].clone())?;
                for operation in operations {
                    media::validate_operation(&operation)?;
                }
            }
            presets.as_array_mut().unwrap().retain(|p| p["id"] != *id);
            presets.as_array_mut().unwrap().push(preset);
        }
        "delete" => presets.as_array_mut().unwrap().retain(|p| p["id"] != *id),
        _ => return Err(usage("unknown presets action")),
    }
    if is_download {
        let mut updated = cfg.clone();
        updated["download_presets"] = presets;
        config::save(&updated)
    } else {
        storage::write_json(&config::directory().join("media_presets.json"), &presets)
    }
}
async fn admin_history(args: &Arguments, cfg: &Value) -> Result<()> {
    let action = args
        .positionals
        .first()
        .map(String::as_str)
        .unwrap_or("list");
    let history = config::history()?;
    let entries = history.as_array().context("invalid history")?;
    if action == "list" {
        let search = args.value("search").to_lowercase();
        return emit(
            &json!(
                entries
                    .iter()
                    .enumerate()
                    .filter(|(_, e)| e.to_string().to_lowercase().contains(&search))
                    .map(|(index, e)| {
                        let mut value = e.clone();
                        value["index"] = json!(index);
                        value
                    })
                    .collect::<Vec<_>>()
            ),
            args.boolean("json"),
        );
    }
    if action == "clear" {
        ensure!(
            args.boolean("yes") || args.boolean("y"),
            "use --yes to clear history"
        );
        return storage::write_json(&config::directory().join("history.json"), &json!([]));
    }
    let index: usize = args
        .positionals
        .get(1)
        .context("history index required")?
        .parse()?;
    let entry = entries.get(index).context("history index out of range")?;
    match action {
        "show" => emit(entry, args.boolean("json")),
        "retry" => {
            let commands: Vec<Vec<String>> =
                serde_json::from_value(entry.get("commands").cloned().unwrap_or(json!([])))?;
            ensure!(!commands.is_empty(), "history entry has no saved commands");
            let queue = Queue::new(number(cfg, "bg_queue_max", 3.0) as usize, None)?;
            let id = queue.enqueue(
                Plan {
                    commands,
                    target: string(entry, "target"),
                    replacement: None,
                    external: None,
                },
                &string(entry, "type"),
                &string(entry, "source"),
                "",
            )?;
            wait_local(&queue, &[id], args).await
        }
        "delete" => {
            crate::daemon::delete_history(entry, args.boolean("delete-file"), cfg)?;
            Ok(())
        }
        _ => Err(usage("unknown history action")),
    }
}
pub fn batch_report(
    inputs: &[String],
    pipeline: &Pipeline,
    output_dir: &str,
    template: &str,
    cfg: &Value,
) -> Result<(Value, Vec<Plan>)> {
    let mut rows = Vec::new();
    let mut plans = Vec::new();
    let mut outputs = BTreeSet::new();
    for input in inputs {
        let mut item = pipeline.clone();
        item.inputs = vec![input.clone()];
        let ext = item
            .operations
            .iter()
            .find(|o| o.kind == "transcode")
            .map(|o| string(&o.config, "container"))
            .filter(|s| !s.is_empty())
            .unwrap_or("mp4".into());
        let stem = Path::new(input)
            .file_stem()
            .unwrap_or_default()
            .to_string_lossy();
        let name = if template.is_empty() {
            format!("{stem}_converted.{ext}")
        } else {
            template
                .replace("%(basename)s", &stem)
                .replace("%(ext)s", &ext)
        };
        let directory = if output_dir.is_empty() {
            Path::new(input)
                .parent()
                .unwrap_or(Path::new("."))
                .to_path_buf()
        } else {
            config::user_path(output_dir)
        };
        item.output.path = media::text_path(&directory.join(name));
        let destination = std::path::absolute(&item.output.path)?;
        let result = if !outputs.insert(destination) {
            Err(anyhow::anyhow!("output name collision"))
        } else {
            media::build(&item, cfg)
        };
        match result{Ok(plan)=>{rows.push(json!({"input":input,"output":plan.target,"valid":true}));plans.push(plan);},Err(error)=>rows.push(json!({"input":input,"output":item.output.path,"valid":false,"error":error.to_string()}))}
    }
    let invalid = rows.iter().filter(|r| r["valid"] == false).count();
    Ok((
        json!({"items":rows,"valid_count":plans.len(),"invalid_count":invalid}),
        plans,
    ))
}
async fn batch(args: &Arguments, cfg: &Value) -> Result<()> {
    let mut inputs = args.positionals.clone();
    inputs.extend(args.all("input"));
    if args.has("directory") {
        for entry in std::fs::read_dir(config::user_path(&args.value("directory")))? {
            let path = entry?.path();
            if path.is_file() {
                inputs.push(media::text_path(&path));
            }
        }
    }
    ensure!(!inputs.is_empty(), "batch inputs required");
    let mut build_args = args.clone();
    build_args.positionals = vec![inputs[0].clone()];
    build_args.flags.remove("input");
    let pipeline = make_pipeline("batch", &build_args)?;
    let (report, plans) = batch_report(
        &inputs,
        &pipeline,
        &args.value("output-dir"),
        &args.value("output-template"),
        cfg,
    )?;
    emit(&report, args.boolean("json"))?;
    if args.boolean("dry-run") {
        return Ok(());
    }
    ensure!(
        report["invalid_count"] == 0 || args.boolean("skip-invalid"),
        "batch validation failed; use --skip-invalid to run valid inputs"
    );
    run_plans(
        plans
            .into_iter()
            .map(|p| (p, "batch".into(), inputs.join(",")))
            .collect(),
        args,
        cfg,
    )
    .await
}
pub fn ranges_plans(args: &Arguments, cfg: &Value) -> Result<Vec<Plan>> {
    let input = args
        .positionals
        .first()
        .cloned()
        .unwrap_or(args.value("input"));
    ensure!(!input.is_empty(), "input required");
    let ranges = args.all("range");
    ensure!(!ranges.is_empty(), "--range START..END required");
    let mut plans = Vec::new();
    let mut outputs = Vec::new();
    for (index, range) in ranges.iter().enumerate() {
        let (start, end) = range.split_once("..").context("range must be START..END")?;
        let output = if ranges.len() == 1 && !args.value("output").is_empty() {
            args.value("output")
        } else if ranges.len() == 1 && !args.value("o").is_empty() {
            args.value("o")
        } else {
            media::unique_output(&input, &format!("_range_{}", index + 1), "mp4", "")
        };
        let pipeline = Pipeline {
            inputs: vec![input.clone()],
            operations: vec![media::operation(
                "trim",
                json!({"start":start,"end":end,"mode":"reencode"}),
            )],
            output: media::Output {
                path: output.clone(),
                overwrite: args.boolean("overwrite"),
                ..Default::default()
            },
            ..Default::default()
        };
        plans.push(media::build(&pipeline, cfg)?);
        outputs.push(output);
    }
    if args.boolean("join") {
        let target = if args.has("o") {
            args.value("o")
        } else if args.has("output") {
            args.value("output")
        } else {
            media::unique_output(&input, "_ranges_joined", "mp4", "")
        };
        media::protect_output(
            &target,
            std::slice::from_ref(&input),
            args.boolean("overwrite"),
        )?;
        let list_path = config::directory()
            .join("concat")
            .join(format!("ranges-{}.ffconcat", media::timestamp()));
        let mut text = String::from("ffconcat version 1.0\n");
        for output in &outputs {
            let absolute = std::path::absolute(output)?;
            let path = media::text_path(&absolute);
            ensure!(!path.contains(['\r', '\n']), "invalid range output path");
            text.push_str(&format!("file '{}'\n", path.replace('\'', "'\\''")));
        }
        if !args.boolean("dry-run") {
            storage::write_private(&list_path, text.as_bytes())?;
        }
        let mut commands: Vec<Vec<String>> = plans.into_iter().flat_map(|p| p.commands).collect();
        commands.push(vec![
            config::tool(cfg, "ffmpeg"),
            "-nostdin".into(),
            if args.boolean("overwrite") {
                "-y".into()
            } else {
                "-n".into()
            },
            "-f".into(),
            "concat".into(),
            "-safe".into(),
            "0".into(),
            "-i".into(),
            media::text_path(&list_path),
            "-c".into(),
            "copy".into(),
            target.clone(),
        ]);
        return Ok(vec![Plan {
            commands,
            target,
            replacement: None,
            external: None,
        }]);
    }
    Ok(plans)
}
async fn ranges(args: &Arguments, cfg: &Value) -> Result<()> {
    let plans = ranges_plans(args, cfg)?;
    run_plans(
        plans
            .into_iter()
            .map(|plan| {
                (
                    plan,
                    "ranges".into(),
                    args.positionals.first().cloned().unwrap_or_default(),
                )
            })
            .collect(),
        args,
        cfg,
    )
    .await
}
