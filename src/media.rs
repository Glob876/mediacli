use crate::{
    config::{self, boolean, number, string},
    storage,
};
use anyhow::{Context, Result, bail, ensure};
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};
use std::{
    fs,
    io::Read,
    path::Path,
    process::{Command, Stdio},
    thread,
    time::{Duration, Instant},
};

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(default)]
pub struct Pipeline {
    pub id: String,
    pub name: String,
    pub inputs: Vec<String>,
    pub operations: Vec<Operation>,
    pub output: Output,
}
#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(default)]
pub struct Output {
    pub path: String,
    pub overwrite: bool,
    pub dir: String,
    pub target_megabytes: f64,
    pub two_pass: bool,
}
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Operation {
    #[serde(rename = "type")]
    pub kind: String,
    pub config: Value,
}
#[derive(Clone, Debug, Default, Serialize, Deserialize)]
pub struct Plan {
    pub commands: Vec<Vec<String>>,
    pub target: String,
    #[serde(default)]
    pub replacement: Option<Replacement>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub external: Option<ExternalTranscode>,
    #[serde(default)]
    pub duration_seconds: f64,
}
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ExternalTranscode {
    pub preset: String,
    pub fields: Value,
    pub config: Value,
}
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Replacement {
    pub input: String,
    pub temporary: String,
    pub final_path: String,
}

pub fn operation(kind: &str, options: Value) -> Operation {
    Operation {
        kind: kind.into(),
        config: options,
    }
}
pub fn timestamp() -> String {
    uuid::Uuid::new_v4().to_string()
}
pub fn text_path(path: &Path) -> String {
    path.to_string_lossy().into_owned()
}
pub fn parse_time(raw: &str) -> Result<f64> {
    let parts: Vec<_> = raw.trim().split(':').collect();
    ensure!(
        !raw.trim().is_empty() && parts.len() <= 3,
        "expected seconds or HH:MM:SS"
    );
    let mut total = 0.0;
    for part in parts {
        let value: f64 = part.parse().context("invalid time")?;
        ensure!(value.is_finite() && value >= 0.0, "invalid time");
        total = total * 60.0 + value;
    }
    Ok(total)
}

pub fn capture(command: &[String], timeout: Duration) -> Result<Vec<u8>> {
    ensure!(!command.is_empty(), "empty command");
    let mut child = Command::new(&command[0])
        .args(&command[1..])
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .with_context(|| format!("Start {}", command[0]))?;
    let mut stdout = child.stdout.take().unwrap();
    let mut stderr = child.stderr.take().unwrap();
    let output_thread = thread::spawn(move || {
        let mut bytes = Vec::new();
        stdout.read_to_end(&mut bytes).map(|_| bytes)
    });
    let error_thread = thread::spawn(move || {
        let mut bytes = Vec::new();
        stderr.read_to_end(&mut bytes).map(|_| bytes)
    });
    let start = Instant::now();
    let mut timed_out = false;
    let status = loop {
        if let Some(status) = child.try_wait()? {
            break status;
        }
        if start.elapsed() >= timeout {
            timed_out = true;
            child.kill()?;
            break child.wait()?;
        }
        thread::sleep(Duration::from_millis(20));
    };
    let output = output_thread
        .join()
        .map_err(|_| anyhow::anyhow!("stdout reader failed"))??;
    let errors = error_thread
        .join()
        .map_err(|_| anyhow::anyhow!("stderr reader failed"))??;
    ensure!(!timed_out, "tool timed out");
    ensure!(
        status.success(),
        "{}: {}",
        command[0],
        String::from_utf8_lossy(&errors)
    );
    Ok(output)
}

pub fn probe(input: &str, cfg: &Value) -> Result<Value> {
    let path = config::user_path(input);
    ensure!(path.is_file(), "input is not a file: {}", path.display());
    let bytes = capture(
        &[
            config::tool(cfg, "ffprobe"),
            "-v".into(),
            "error".into(),
            "-show_format".into(),
            "-show_streams".into(),
            "-show_chapters".into(),
            "-of".into(),
            "json".into(),
            text_path(&path),
        ],
        Duration::from_secs(20),
    )?;
    let mut info: Value = serde_json::from_slice(&bytes)?;
    info["path"] = json!(path);
    let duration = number(&info["format"], "duration", 0.0);
    info["format"]["duration_sec"] = json!(duration);
    if let Some(streams) = info["streams"].as_array_mut() {
        for stream in streams {
            let rate = string(stream, "avg_frame_rate");
            let pieces: Vec<_> = rate.split('/').collect();
            let fps = if pieces.len() == 2 {
                pieces[0].parse::<f64>().unwrap_or(0.0) / pieces[1].parse::<f64>().unwrap_or(1.0)
            } else {
                0.0
            };
            stream["fps"] = json!(if fps.is_finite() { fps } else { 0.0 });
        }
    }
    Ok(info)
}

pub fn unique_output(input: &str, suffix: &str, extension: &str, dir: &str) -> String {
    let path = config::user_path(input);
    let parent = if dir.is_empty() {
        path.parent().unwrap_or(Path::new(".")).to_path_buf()
    } else {
        config::user_path(dir)
    };
    let stem = path.file_stem().unwrap_or_default().to_string_lossy();
    let base = parent.join(format!("{stem}{suffix}"));
    let mut output = base.with_extension(extension.trim_start_matches('.'));
    let mut index = 2;
    while output.exists() {
        output = parent.join(format!(
            "{stem}{suffix}_{index}.{}",
            extension.trim_start_matches('.')
        ));
        index += 1;
    }
    text_path(&output)
}

pub fn protect_output(output: &str, inputs: &[String], overwrite: bool) -> Result<()> {
    let output_path = std::path::absolute(config::user_path(output))?;
    for input in inputs {
        let input_path = std::path::absolute(config::user_path(input))?;
        ensure!(
            output_path != input_path
                && !same_file::is_same_file(&output_path, &input_path).unwrap_or(false),
            "output overwrites input: {input}"
        );
    }
    ensure!(
        overwrite || !output_path.exists(),
        "output already exists: {output}"
    );
    Ok(())
}

pub fn validate_operation(op: &Operation) -> Result<()> {
    let schema = config::schema();
    let fields = schema[&op.kind]
        .as_object()
        .with_context(|| format!("unknown operation: {}", op.kind))?;
    for (key, value) in op
        .config
        .as_object()
        .context("operation config must be an object")?
    {
        let expected = fields
            .get(key)
            .and_then(Value::as_str)
            .with_context(|| format!("unknown {} field: {key}", op.kind))?;
        let valid = match expected {
            "string" => value.is_string(),
            "int" => value.is_i64(),
            "bool" => value.is_boolean(),
            "*float64" => value.is_number() || value.is_null(),
            "[]string" => value
                .as_array()
                .is_some_and(|a| a.iter().all(Value::is_string)),
            "map[string]string" => value
                .as_object()
                .is_some_and(|o| o.values().all(Value::is_string)),
            _ => false,
        };
        ensure!(
            valid,
            "invalid type for {}.{key}: expected {expected}",
            op.kind
        );
    }
    Ok(())
}

pub fn preset_pipeline(id: &str, input: &str) -> Result<Pipeline> {
    let presets = config::convert_presets();
    let alias = match id {
        "mkv_av1" => "modern_mkv",
        "davinci_dnxhr" => "davinci_dnxhr_hq",
        other => other,
    };
    let preset = presets
        .as_array()
        .unwrap()
        .iter()
        .find(|p| p["id"] == alias)
        .with_context(|| format!("unknown conversion preset: {id}"))?;
    let flags: Vec<String> = serde_json::from_value(preset["flags"].clone())?;
    let mut options = json!({"container":preset["ext"],"extra":flags});
    // Preset flags are passed after codec defaults, preserving their precedence.
    if id.starts_with("audio_") {
        options["vcodec"] = json!("");
    }
    Ok(Pipeline {
        inputs: vec![input.into()],
        operations: vec![operation("transcode", options)],
        ..Default::default()
    })
}

pub fn build(p: &Pipeline, cfg: &Value) -> Result<Plan> {
    ensure!(!p.inputs.is_empty(), "no inputs");
    let mut inputs: Vec<String> = p
        .inputs
        .iter()
        .map(|s| text_path(&config::user_path(s)))
        .collect();
    for input in &inputs {
        ensure!(Path::new(input).is_file(), "input not found: {input}");
    }
    for op in &p.operations {
        validate_operation(op)?;
    }
    let info = probe(&inputs[0], cfg)?;
    let streams = info["streams"].as_array().context("missing streams")?;
    let has_video = streams.iter().any(|s| s["codec_type"] == "video");
    let has_audio = streams.iter().any(|s| s["codec_type"] == "audio");
    let mut extension = if has_video {
        "mp4".to_owned()
    } else {
        "m4a".to_owned()
    };
    for op in &p.operations {
        let field = if op.kind == "demux" {
            "output_ext"
        } else {
            "container"
        };
        let candidate = string(&op.config, field);
        if !candidate.is_empty() {
            extension = candidate.trim_start_matches('.').to_string();
        }
        if op.kind == "subtitle" && op.config["mode"] == "extract" {
            extension = "srt".into();
        }
        if op.kind == "trim" && op.config["mode"] == "copy" {
            extension = Path::new(&inputs[0])
                .extension()
                .unwrap_or_default()
                .to_string_lossy()
                .into_owned();
        }
    }
    ensure!(
        [
            "mp4", "mkv", "mov", "webm", "gif", "avi", "m4v", "mp3", "flac", "wav", "m4a", "opus",
            "ogg", "srt", "vtt", "ass", "aac", "h264", "hevc"
        ]
        .contains(&extension.as_str()),
        "unsupported output container: {extension}"
    );
    let mut sources = inputs.clone();
    for op in &p.operations {
        for key in ["inputs", "extra_inputs"] {
            if let Some(paths) = op.config[key].as_array() {
                for path in paths {
                    sources.push(path.as_str().context("path must be a string")?.into());
                }
            }
        }
        for key in ["input", "cover"] {
            let path = string(&op.config, key);
            if !path.is_empty() {
                sources.push(path);
            }
        }
    }
    let requested = if p.output.path.is_empty() {
        unique_output(&inputs[0], "_pipelined", &extension, &p.output.dir)
    } else {
        text_path(&config::user_path(&p.output.path))
    };
    protect_output(&requested, &sources, p.output.overwrite)?;
    let mut video = if has_video {
        "copy".to_string()
    } else {
        String::new()
    };
    let mut audio = if has_audio {
        "copy".to_string()
    } else {
        String::new()
    };
    let mut args: Vec<String> = Vec::new();
    let mut input_args: Vec<String> = Vec::new();
    let mut video_filters: Vec<String> = Vec::new();
    let mut audio_filters: Vec<String> = Vec::new();
    let mut complex = String::new();
    let mut mapped_video = String::new();
    let mut mapped_audio = String::new();
    let mut passes: Vec<Vec<String>> = Vec::new();
    let mut duration = number(&info["format"], "duration", 0.0);
    let mut explicit_video_copy = false;
    let mut explicit_audio_copy = false;
    for (op_index, op) in p.operations.iter().enumerate() {
        let options = &op.config;
        let s = |key: &str| string(options, key);
        let b = |key: &str| boolean(options, key);
        let n = |key: &str, default: f64| number(options, key, default);
        match op.kind.as_str() {
            "transcode" => {
                if b("remux") {
                    video = "copy".into();
                    audio = "copy".into();
                    explicit_video_copy = true;
                    explicit_audio_copy = true;
                }
                for (key, target, explicit) in [
                    ("vcodec", &mut video, &mut explicit_video_copy),
                    ("acodec", &mut audio, &mut explicit_audio_copy),
                ] {
                    let codec = s(key);
                    if !codec.is_empty() {
                        *explicit = codec == "copy";
                        *target = codec;
                    }
                }
                for (key, flag) in [
                    ("crf", "-crf"),
                    ("preset", "-preset"),
                    ("bitrate_v", "-b:v"),
                    ("bitrate_a", "-b:a"),
                    ("pix_fmt", "-pix_fmt"),
                    ("profile", "-profile:v"),
                    ("maxrate", "-maxrate"),
                    ("bufsize", "-bufsize"),
                    ("gop", "-g"),
                ] {
                    let value = s(key);
                    if !value.is_empty() {
                        args.extend([flag.into(), value]);
                    }
                }
                if let Some(extra) = options["extra"].as_array() {
                    args.extend(extra.iter().map(|v| v.as_str().unwrap().to_string()));
                }
            }
            "trim" => {
                ensure!(
                    !s("start").is_empty() || !s("end").is_empty(),
                    "trim requires start or end"
                );
                let start = if s("start").is_empty() {
                    0.0
                } else {
                    parse_time(&s("start"))?
                };
                let end = if s("end").is_empty() {
                    duration
                } else {
                    parse_time(&s("end"))?
                };
                ensure!(
                    end > start && (duration <= 0.0 || start < duration),
                    "invalid trim interval"
                );
                args.extend([
                    "-ss".into(),
                    start.to_string(),
                    "-t".into(),
                    (end - start).to_string(),
                ]);
                duration = end - start;
                ensure!(
                    ["", "copy", "reencode"].contains(&s("mode").as_str()),
                    "invalid trim mode"
                );
                if s("mode") == "copy" {
                    video = "copy".into();
                    audio = "copy".into();
                    explicit_video_copy = true;
                    explicit_audio_copy = true;
                } else {
                    if video == "copy" {
                        video = "libx264".into();
                    }
                    if audio == "copy" {
                        audio = "aac".into();
                    }
                }
            }
            "scale" => {
                ensure!(has_video, "scale requires video");
                let width = n("width", -2.0) as i64;
                let height = n("height", -2.0) as i64;
                ensure!(width > 0 || height > 0, "scale requires width or height");
                let algorithm = if s("algo").is_empty() {
                    "lanczos".into()
                } else {
                    s("algo")
                };
                video_filters.push(format!("scale={width}:{height}:flags={algorithm}"));
            }
            "fps" => {
                ensure!(
                    has_video && !s("fps").is_empty(),
                    "fps requires video and a frame rate"
                );
                let filter = if b("interpolate") {
                    "minterpolate"
                } else {
                    "fps"
                };
                video_filters.push(format!("{filter}=fps={}", s("fps")));
                if s("mode") == "r" {
                    args.extend(["-r".into(), s("fps")]);
                }
            }
            "filter" => {
                ensure!(
                    !s("vf").is_empty() || !s("af").is_empty() || !s("complex").is_empty(),
                    "empty filter"
                );
                if !s("vf").is_empty() {
                    video_filters.push(s("vf"));
                }
                if !s("af").is_empty() {
                    audio_filters.push(s("af"));
                }
                if !s("complex").is_empty() {
                    ensure!(
                        complex.is_empty(),
                        "multiple custom complex graphs are ambiguous"
                    );
                    complex = s("complex");
                }
            }
            "aspect" => {
                ensure!(has_video, "aspect requires video");
                let ratio_w = n("ratio_w", 0.0);
                let ratio_h = n("ratio_h", 0.0);
                ensure!(ratio_w > 0.0 && ratio_h > 0.0, "ratio must be positive");
                let base = match s("size_preset").as_str() {
                    "720p" => 720.0,
                    "4k" => 2160.0,
                    other => other.trim_end_matches('p').parse::<f64>().unwrap_or(1080.0),
                };
                let mut width = n("width", 0.0) as i64;
                let mut height = n("height", 0.0) as i64;
                if width <= 0 && height <= 0 {
                    if ratio_w >= ratio_h {
                        height = base as i64;
                        width = (base * ratio_w / ratio_h).round() as i64;
                    } else {
                        width = base as i64;
                        height = (base * ratio_h / ratio_w).round() as i64;
                    }
                }
                if width <= 0 {
                    width = (height as f64 * ratio_w / ratio_h).round() as i64;
                }
                if height <= 0 {
                    height = (width as f64 * ratio_h / ratio_w).round() as i64;
                }
                width += width % 2;
                height += height % 2;
                let mode = s("mode");
                match mode.as_str(){
                    ""|"crop"=>video_filters.push(format!("scale={width}:{height}:force_original_aspect_ratio=increase,crop={width}:{height},setsar=1")),
                    "letterbox"=>{let color=if s("pad_color").is_empty(){"black".into()}else{s("pad_color")};video_filters.push(format!("scale={width}:{height}:force_original_aspect_ratio=decrease,pad={width}:{height}:(ow-iw)/2:(oh-ih)/2:color={color},setsar=1"));},
                    "stretch"=>video_filters.push(format!("scale={width}:{height},setsar=1")),
                    "blur"=>{let label=format!("blur{op_index}");video_filters.push(format!("split[{label}bg][{label}fg];[{label}bg]scale={width}:{height}:force_original_aspect_ratio=increase,crop={width}:{height},gblur=sigma=20[{label}back];[{label}fg]scale={width}:{height}:force_original_aspect_ratio=decrease[{label}front];[{label}back][{label}front]overlay=(W-w)/2:(H-h)/2,setsar=1"));},
                    "rotate"|"flip"=>{},_=>bail!("unknown aspect mode")
                }
                match n("rotate", 0.0) as i64 {
                    90 => video_filters.push("transpose=1".into()),
                    180 => video_filters.push("hflip,vflip".into()),
                    270 => video_filters.push("transpose=2".into()),
                    0 => {}
                    _ => bail!("rotation must be 0, 90, 180 or 270"),
                }
                if b("flip_h") {
                    video_filters.push("hflip".into());
                }
                if b("flip_v") {
                    video_filters.push("vflip".into());
                }
                if !s("codec").is_empty() {
                    video = match s("codec").as_str() {
                        "h264" => "libx264",
                        "hevc" => "libx265",
                        "prores" => "prores_ks",
                        "dnxhr" => "dnxhd",
                        _ => bail!("unknown aspect codec"),
                    }
                    .into();
                }
            }
            "audio" => {
                ensure!(has_audio, "audio operation requires audio");
                if !options["volume"].is_null() {
                    let volume = n("volume", 1.0);
                    ensure!(volume >= 0.0 && volume.is_finite(), "invalid volume");
                    audio_filters.push(format!("volume={volume}"));
                }
                if b("normalize") {
                    audio_filters.push("loudnorm=I=-16:TP=-1.5:LRA=11".into());
                }
                if !s("tempo").is_empty() {
                    audio_filters.push(format!("atempo={}", s("tempo")));
                }
                if b("remove_silence") {
                    audio_filters.push(
                        "silenceremove=start_periods=1:start_duration=0.5:start_threshold=-60dB"
                            .into(),
                    );
                }
                if !s("fade_in").is_empty() {
                    audio_filters.push(format!("afade=t=in:st=0:d={}", s("fade_in")));
                }
                if !s("fade_out").is_empty() {
                    let seconds = parse_time(&s("fade_out"))?;
                    ensure!(duration > 0.0, "fade out requires known duration");
                    audio_filters.push(format!(
                        "afade=t=out:st={}:d={seconds}",
                        (duration - seconds).max(0.0)
                    ));
                }
                for (key, flag) in [
                    ("sample_rate", "-ar"),
                    ("channels", "-ac"),
                    ("bitrate", "-b:a"),
                ] {
                    if !s(key).is_empty() {
                        args.extend([flag.into(), s(key)]);
                    }
                }
                if !s("acodec").is_empty() {
                    audio = s("acodec");
                    explicit_audio_copy = audio == "copy";
                }
            }
            "mux" => {
                let extra: Vec<String> = serde_json::from_value(
                    options.get("extra_inputs").cloned().unwrap_or(json!([])),
                )?;
                ensure!(!extra.is_empty(), "mux requires extra inputs");
                let first_extra = inputs.len();
                inputs.extend(extra);
                if b("mix_audio") {
                    ensure!(complex.is_empty(), "incompatible complex filters");
                    complex =
                        format!("[0:a][{first_extra}:a]amix=inputs=2:duration=longest[mixed]");
                    mapped_audio = "[mixed]".into();
                    audio = "aac".into();
                    args.extend(["-map".into(), "0:v:0?".into()]);
                } else if let Some(maps) = options["maps"].as_array().filter(|a| !a.is_empty()) {
                    for map in maps {
                        args.extend(["-map".into(), map.as_str().context("invalid map")?.into()]);
                    }
                } else {
                    args.extend(["-map".into(), "0:v:0?".into()]);
                    for index in first_extra..inputs.len() {
                        args.extend(["-map".into(), format!("{index}:a:0?")]);
                    }
                }
                if b("shortest") {
                    args.push("-shortest".into());
                }
                if !s("vcodec").is_empty() {
                    video = s("vcodec");
                }
                if !s("acodec").is_empty() {
                    audio = s("acodec");
                }
            }
            "demux" => {
                let index = n("stream_index", 0.0) as i64;
                ensure!(index >= 0, "negative stream index");
                let codec = if s("codec").is_empty() {
                    "copy".into()
                } else {
                    s("codec")
                };
                match s("stream_type").as_str() {
                    "audio" => {
                        video.clear();
                        audio = codec;
                        args.extend(["-map".into(), format!("0:a:{index}"), "-vn".into()]);
                    }
                    "video" => {
                        audio.clear();
                        video = codec;
                        args.extend(["-map".into(), format!("0:v:{index}"), "-an".into()]);
                    }
                    "subtitle" => {
                        video.clear();
                        audio.clear();
                        args.extend([
                            "-map".into(),
                            format!("0:s:{index}"),
                            "-c:s".into(),
                            "srt".into(),
                        ]);
                    }
                    _ => bail!("unknown stream_type"),
                }
            }
            "subtitle" => {
                let index = n("index", 0.0) as i64;
                ensure!(index >= 0, "negative subtitle index");
                match s("mode").as_str() {
                    "extract" => {
                        video.clear();
                        audio.clear();
                        args.extend([
                            "-map".into(),
                            format!("0:s:{index}"),
                            "-c:s".into(),
                            "srt".into(),
                        ]);
                    }
                    "embed" => {
                        if !s("input").is_empty() {
                            let index = inputs.len();
                            inputs.push(s("input"));
                            args.extend([
                                "-map".into(),
                                "0".into(),
                                "-map".into(),
                                format!("{index}:s:0"),
                            ]);
                        }
                        let codec = if extension == "mp4" || extension == "mov" {
                            "mov_text"
                        } else {
                            "srt"
                        };
                        args.extend(["-c:s".into(), codec.into()]);
                        if !s("language").is_empty() {
                            args.extend([
                                "-metadata:s:s:0".into(),
                                format!("language={}", s("language")),
                            ]);
                        }
                    }
                    "burn" => {
                        ensure!(!s("input").is_empty(), "burn requires subtitle input");
                        let path = filter_escape(&s("input"));
                        let style = if s("style").is_empty() {
                            String::new()
                        } else {
                            format!(":force_style='{}'", filter_escape(&s("style")))
                        };
                        video_filters.push(format!("subtitles=filename='{path}'{style}"));
                    }
                    _ => bail!("unknown subtitle mode"),
                }
            }
            "metadata" => {
                if b("strip_all") {
                    args.extend(["-map_metadata".into(), "-1".into()]);
                }
                for key in ["title", "artist", "album", "comment"] {
                    if !s(key).is_empty() {
                        args.extend(["-metadata".into(), format!("{key}={}", s(key))]);
                    }
                }
                if let Some(extra) = options["extra"].as_object() {
                    for (key, value) in extra {
                        args.extend([
                            "-metadata".into(),
                            format!("{key}={}", value.as_str().unwrap()),
                        ]);
                    }
                }
                if !s("cover").is_empty() {
                    let index = inputs.len();
                    inputs.push(s("cover"));
                    args.extend([
                        "-map".into(),
                        "0".into(),
                        "-map".into(),
                        format!("{index}:v:0"),
                        "-c:v:1".into(),
                        "mjpeg".into(),
                        "-disposition:v:1".into(),
                        "attached_pic".into(),
                    ]);
                }
            }
            "concat" => {
                let items: Vec<String> = serde_json::from_value(options["inputs"].clone())?;
                ensure!(items.len() >= 2, "concat requires at least two files");
                for item in &items {
                    ensure!(
                        config::user_path(item).is_file(),
                        "concat input not found: {item}"
                    );
                }
                if s("mode") == "filter" {
                    ensure!(complex.is_empty(), "incompatible complex filters");
                    inputs = items;
                    let mut labels = String::new();
                    for index in 0..inputs.len() {
                        if has_video {
                            labels.push_str(&format!("[{index}:v]"));
                        }
                        if has_audio {
                            labels.push_str(&format!("[{index}:a]"));
                        }
                    }
                    complex = format!(
                        "{labels}concat=n={}:v={}:a={}",
                        inputs.len(),
                        usize::from(has_video),
                        usize::from(has_audio)
                    );
                    if has_video {
                        complex.push_str("[concatenated_v]");
                        mapped_video = "[concatenated_v]".into();
                        video = if s("codec").is_empty() {
                            "libx264".into()
                        } else {
                            s("codec")
                        };
                    }
                    if has_audio {
                        complex.push_str("[concatenated_a]");
                        mapped_audio = "[concatenated_a]".into();
                        audio = "aac".into();
                    }
                } else {
                    ensure!(
                        ["", "demuxer"].contains(&s("mode").as_str()),
                        "unknown concat mode"
                    );
                    let mut list = String::from("ffconcat version 1.0\n");
                    for item in items {
                        let path = text_path(&fs::canonicalize(config::user_path(&item))?);
                        ensure!(!path.contains(['\r', '\n']), "concat path contains newline");
                        list.push_str(&format!("file '{}'\n", path.replace('\'', "'\\''")));
                    }
                    let path = config::directory()
                        .join("concat")
                        .join(format!("{}.ffconcat", timestamp()));
                    storage::write_private(&path, list.as_bytes())?;
                    inputs = vec![text_path(&path)];
                    input_args.extend(["-f".into(), "concat".into(), "-safe".into(), "0".into()]);
                }
            }
            "stabilize" => {
                ensure!(has_video, "stabilize requires video");
                let transform = config::directory()
                    .join("cache")
                    .join(format!("{}.trf", timestamp()));
                fs::create_dir_all(transform.parent().unwrap())?;
                let path = filter_escape(&text_path(&transform));
                passes.push(vec![
                    config::tool(cfg, "ffmpeg"),
                    "-nostdin".into(),
                    "-y".into(),
                    "-i".into(),
                    inputs[0].clone(),
                    "-vf".into(),
                    format!(
                        "vidstabdetect=shakiness={}:accuracy={}:result='{path}'",
                        n("shakiness", 10.0),
                        n("accuracy", 15.0)
                    ),
                    "-an".into(),
                    "-f".into(),
                    "null".into(),
                    "-".into(),
                ]);
                video_filters.push(format!("vidstabtransform=input='{path}'"));
            }
            _ => bail!("unknown operation: {}", op.kind),
        }
    }
    if !video_filters.is_empty() {
        ensure!(has_video, "video filter requires video");
        ensure!(!explicit_video_copy, "video filters require re-encoding");
        if video == "copy" {
            video = "libx264".into();
        }
    }
    if !audio_filters.is_empty() {
        ensure!(has_audio, "audio filter requires audio");
        ensure!(!explicit_audio_copy, "audio filters require re-encoding");
        if audio == "copy" {
            audio = "aac".into();
        }
    }
    if !complex.is_empty() {
        if !video_filters.is_empty() {
            ensure!(
                !mapped_video.is_empty(),
                "custom complex graph cannot be combined with video filters without an output mapping"
            );
            complex.push_str(&format!(
                ";{}{}[final_v]",
                mapped_video,
                video_filters.join(",")
            ));
            mapped_video = "[final_v]".into();
        }
        if !audio_filters.is_empty() {
            let label = if mapped_audio.is_empty() {
                "[0:a]"
            } else {
                &mapped_audio
            };
            complex.push_str(&format!(";{label}{}[final_a]", audio_filters.join(",")));
            mapped_audio = "[final_a]".into();
        }
        args.extend(["-filter_complex".into(), complex]);
        if !mapped_video.is_empty() {
            args.extend(["-map".into(), mapped_video]);
        }
        if !mapped_audio.is_empty() {
            args.extend(["-map".into(), mapped_audio]);
        }
    } else {
        if !video_filters.is_empty() {
            args.extend(["-vf".into(), video_filters.join(",")]);
        }
        if !audio_filters.is_empty() {
            args.extend(["-af".into(), audio_filters.join(",")]);
        }
    }
    let mut command = vec![
        config::tool(cfg, "ffmpeg"),
        "-nostdin".into(),
        if p.output.overwrite {
            "-y".into()
        } else {
            "-n".into()
        },
    ];
    command.extend(input_args);
    for input in inputs {
        command.extend(["-i".into(), input]);
    }
    if !has_video {
        video.clear();
    }
    if !has_audio {
        audio.clear();
    }
    if !video.is_empty() {
        command.extend(["-c:v".into(), video]);
    }
    if !audio.is_empty() {
        command.extend(["-c:a".into(), audio]);
    }
    command.extend(args);
    command.push(requested.clone());
    if p.output.target_megabytes > 0.0 {
        ensure!(
            has_video && duration > 0.0 && p.output.target_megabytes.is_finite(),
            "size export requires video with a known duration"
        );
        let audio_bits = if has_audio { 128000.0 } else { 0.0 };
        let bits = p.output.target_megabytes * 1_000_000.0 * 8.0 * 0.96 / duration - audio_bits;
        ensure!(bits >= 32000.0, "size too small for duration");
        command.pop();
        command.extend([
            "-c:v".into(),
            "libx264".into(),
            "-b:v".into(),
            format!("{bits:.0}"),
            "-pix_fmt".into(),
            "yuv420p".into(),
            "-c:a".into(),
            "aac".into(),
            "-b:a".into(),
            "128k".into(),
        ]);
        // CRF would override the target bitrate; remove it from every operation.
        remove_option(&mut command, "-crf");
        if p.output.two_pass {
            ensure!(
                passes.is_empty(),
                "size export cannot combine another multipass operation"
            );
            let log =
                text_path(&std::env::temp_dir().join(format!("mediacli-pass-{}", timestamp())));
            let mut first = command.clone();
            first[2] = "-y".into();
            first.extend([
                "-pass".into(),
                "1".into(),
                "-passlogfile".into(),
                log.clone(),
                "-an".into(),
                "-f".into(),
                "null".into(),
                "-".into(),
            ]);
            passes.push(first);
            command.extend(["-pass".into(), "2".into(), "-passlogfile".into(), log]);
        }
        command.push(requested.clone());
    }
    passes.push(command);
    Ok(Plan {
        commands: passes,
        target: requested,
        replacement: None,
        external: None,
        duration_seconds: duration,
    })
}

pub fn remove_option(command: &mut Vec<String>, name: &str) {
    let mut index = 0;
    while index < command.len() {
        if command[index] == name {
            command.remove(index);
            if index < command.len() {
                command.remove(index);
            }
        } else {
            index += 1;
        }
    }
}
pub fn filter_escape(text: &str) -> String {
    text.replace('\\', "\\\\")
        .replace(':', "\\:")
        .replace('\'', "'\\''")
}

impl Replacement {
    pub fn finish(&self, success: bool) -> Result<()> {
        if !success {
            match fs::remove_file(&self.temporary) {
                Ok(()) => {}
                Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
                Err(error) => return Err(error.into()),
            }
            return Ok(());
        }
        let metadata = fs::metadata(&self.temporary)?;
        ensure!(
            metadata.is_file() && metadata.len() > 0,
            "converted file is empty"
        );
        let same = same_file::is_same_file(&self.input, &self.final_path).unwrap_or(false)
            || self.input == self.final_path;
        ensure!(
            same || !Path::new(&self.final_path).exists(),
            "replacement destination appeared during conversion; original and temporary preserved"
        );
        fs::rename(&self.temporary, &self.final_path)
            .context("install converted file; original preserved")?;
        if !same {
            fs::remove_file(&self.input)
                .context("converted file saved; removing original failed")?;
        }
        Ok(())
    }
}

pub fn waveform(input: &str, start: f64, columns: usize, cfg: &Value) -> Result<Vec<f64>> {
    ensure!(
        start.is_finite() && start >= 0.0 && (1..=4096).contains(&columns),
        "invalid waveform window"
    );
    let bytes = capture(
        &vec![
            config::tool(cfg, "ffmpeg"),
            "-nostdin".into(),
            "-v".into(),
            "error".into(),
            "-ss".into(),
            start.to_string(),
            "-i".into(),
            input.into(),
            "-t".into(),
            "60".into(),
            "-vn".into(),
            "-ac".into(),
            "1".into(),
            "-ar".into(),
            "8000".into(),
            "-f".into(),
            "s16le".into(),
            "-".into(),
        ],
        Duration::from_secs(30),
    )?;
    let samples: Vec<f64> = bytes
        .as_chunks::<2>()
        .0
        .iter()
        .map(|b| f64::from(i16::from_le_bytes([b[0], b[1]])).abs() / 32768.0)
        .collect();
    let mut peaks = vec![0.0_f64; columns];
    for (index, sample) in samples.iter().enumerate() {
        let column = index * columns / samples.len();
        peaks[column] = peaks[column].max(*sample);
    }
    Ok(peaks)
}

pub fn masked(command: &[String]) -> Vec<String> {
    let mut result = command.to_vec();
    for index in 0..result.len().saturating_sub(1) {
        if ["--proxy", "--video-password"].contains(&result[index].as_str())
            && !result[index + 1].is_empty()
        {
            result[index + 1] = "***".into();
        }
    }
    result
}
pub fn display_command(command: &[String]) -> String {
    shell_words::join(masked(command))
}

/// Build a conversion without ever writing directly over its source.
pub fn conversion_plan(preset: &str, input: &str, output: &str, cfg: &Value) -> Result<Plan> {
    let mut pipeline = preset_pipeline(preset, input)?;
    if !output.is_empty() {
        pipeline.output.path = output.into();
        return build(&pipeline, cfg);
    }
    let extension = string(&pipeline.operations[0].config, "container");
    if config::boolean(cfg, "overwrite_original") {
        let path = config::user_path(input);
        let final_path = path.with_extension(&extension);
        if final_path.exists() && !same_file::is_same_file(&path, &final_path).unwrap_or(false) {
            bail!(
                "replacement destination already exists: {}",
                final_path.display()
            );
        }
        let stem = path.file_stem().unwrap_or_default().to_string_lossy();
        let temporary = path.with_file_name(format!(".{stem}_tmp_{}.{}", timestamp(), extension));
        pipeline.output.path = text_path(&temporary);
        let mut plan = build(&pipeline, cfg)?;
        plan.target = text_path(&final_path);
        plan.replacement = Some(Replacement {
            input: text_path(&path),
            temporary: text_path(&temporary),
            final_path: text_path(&final_path),
        });
        return Ok(plan);
    }
    let presets = config::convert_presets();
    let suffix = presets
        .as_array()
        .unwrap()
        .iter()
        .find(|p| p["id"] == preset)
        .map(|p| string(p, "suffix"))
        .unwrap_or("_converted".into());
    let same_extension = Path::new(input)
        .extension()
        .and_then(|s| s.to_str())
        .is_some_and(|ext| ext.eq_ignore_ascii_case(&extension));
    let suffix = if boolean(cfg, "use_ffmpeg_suffix") || same_extension {
        suffix
    } else {
        String::new()
    };
    pipeline.output.path = unique_output(input, &suffix, &extension, "");
    build(&pipeline, cfg)
}
