use crate::{
    config::{self, boolean, string},
    media::{self, Plan},
};
use anyhow::{Context, Result, ensure};
use serde_json::Value;
use std::path::Path;

pub fn format_expression(quality: &str, fps: &str, codec: &str) -> String {
    let height = if quality.is_empty() {
        String::new()
    } else {
        format!("[height<={quality}]")
    };
    let rate = if fps.is_empty() {
        String::new()
    } else {
        format!("[fps<={fps}]")
    };
    let video = match codec {
        "av1" => "[vcodec~='^av0?1']",
        "vp9" => "[vcodec~='^vp0?9']",
        "h264" => "[vcodec~='^avc']",
        _ => "",
    };
    if video.is_empty() {
        format!("bv{height}{rate}+ba/b{height}{rate}")
    } else {
        format!(
            "bv{height}{rate}{video}+ba/b{height}{rate}{video}/bv{height}{rate}+ba/b{height}{rate}"
        )
    }
}

pub fn build(
    url: &str,
    fields: &Value,
    cfg: &Value,
    out_dir: &str,
    playlist: bool,
    thumbnail: Option<&str>,
) -> Result<Plan> {
    ensure!(
        url.starts_with("https://") || url.starts_with("http://"),
        "expected an HTTP(S) media URL"
    );
    let mut options = config::download_fields();
    config::merge(&mut options, fields)?;
    let s = |key: &str| string(&options, key);
    let b = |key: &str| boolean(&options, key);
    let fallback = |key: &str| {
        let value = s(key);
        if value.is_empty() || value == "default" {
            string(cfg, key)
        } else {
            value
        }
    };
    let preset_id = fallback("video_preset");
    let external = string(cfg, "transcode_mode") == "external"
        && !b("audio_only")
        && thumbnail.is_none()
        && !["", "default", "audio_mp3", "audio_flac"].contains(&preset_id.as_str());
    let mut command = vec![
        "yt-dlp".into(),
        "--newline".into(),
        "--progress".into(),
        "--no-simulate".into(),
        "--no-color".into(),
        "--continue".into(),
        "--print".into(),
        "after_move:MEDIACLI_FILE=%(filepath)j".into(),
    ];
    command.push(if playlist {
        "--yes-playlist".into()
    } else {
        "--no-playlist".into()
    });
    command.extend([
        "--concurrent-fragments".into(),
        fallback("concurrent_fragments"),
        "--retries".into(),
        if s("retries").is_empty() {
            "10".into()
        } else {
            s("retries")
        },
        "--fragment-retries".into(),
        "10".into(),
    ]);
    for (key, flag) in [
        ("no_mtime", "--no-mtime"),
        ("windows_filenames", "--windows-filenames"),
        ("restrict_filenames", "--restrict-filenames"),
        ("force_overwrite", "--force-overwrites"),
        ("geobypass", "--geo-bypass"),
        ("live_start", "--live-from-start"),
        ("split_chapters", "--split-chapters"),
    ] {
        if b(key) || (["no_mtime", "windows_filenames"].contains(&key) && boolean(cfg, key)) {
            command.push(flag.into());
        }
    }
    if !string(cfg, "ffmpeg_path").is_empty() {
        command.extend(["--ffmpeg-location".into(), config::tool(cfg, "ffmpeg")]);
    }
    for (key, flag) in [
        ("video_password", "--video-password"),
        ("ratelimit", "--limit-rate"),
    ] {
        if !s(key).is_empty() {
            command.extend([flag.into(), s(key)]);
        }
    }
    if !s("download_section").is_empty() {
        let section = s("download_section");
        command.extend([
            "--download-sections".into(),
            if section.starts_with('*') {
                section
            } else {
                format!("*{section}")
            },
        ]);
    }
    if b("use_archive") || boolean(cfg, "use_archive") {
        command.extend(["--download-archive".into(), string(cfg, "archive_file")]);
    }
    match fallback("cookies_mode").as_str() {
        "file" => {
            let file = fallback("cookies_file");
            ensure!(!file.is_empty(), "cookies file is required");
            command.extend([
                "--cookies".into(),
                media::text_path(&config::user_path(&file)),
            ]);
        }
        "browser" => {
            command.extend(["--cookies-from-browser".into(), fallback("cookies_browser")]);
        }
        "none" | "" => {}
        _ => anyhow::bail!("invalid cookies mode"),
    }
    match fallback("proxy_mode").as_str() {
        "custom" => {
            let proxy = fallback("proxy_url");
            ensure!(!proxy.is_empty(), "proxy URL is required");
            command.extend(["--proxy".into(), proxy]);
        }
        "none" => command.extend(["--proxy".into(), String::new()]),
        "system" | "" => {}
        _ => anyhow::bail!("invalid proxy mode"),
    }
    if let Some(format) = thumbnail {
        ensure!(
            ["png", "jpg", "webp"].contains(&format),
            "unsupported thumbnail format"
        );
        command.extend([
            "--skip-download".into(),
            "--write-thumbnail".into(),
            "--convert-thumbnails".into(),
            format.into(),
        ]);
    } else if b("audio_only") {
        command.extend([
            "-x".into(),
            "--audio-format".into(),
            fallback("audio_format"),
            "--audio-quality".into(),
            s("audio_quality"),
        ]);
    } else {
        command.extend([
            "-f".into(),
            format_expression(&s("quality"), &s("fps_limit"), &s("vcodec")),
            "-S".into(),
            "res,ext:mp4:m4a:webm,codec:h264:vp9:av01".into(),
        ]);
        let id = fallback("video_preset");
        if external {
            command.extend(["--merge-output-format".into(), "mp4".into()]);
        } else if id == "custom" {
            let ext = if s("custom_ext").is_empty() {
                "mp4".into()
            } else {
                s("custom_ext")
            };
            let flags = if s("custom_flags").is_empty() {
                "-c:v libx264 -crf 18 -c:a aac".into()
            } else {
                s("custom_flags")
            };
            shell_words::split(&flags).context("invalid custom FFmpeg flags")?;
            command.extend([
                "--recode-video".into(),
                ext,
                "--postprocessor-args".into(),
                format!("ffmpeg:{flags}"),
            ]);
        } else {
            let presets = config::video_presets();
            let preset = presets
                .as_array()
                .unwrap()
                .iter()
                .find(|p| p["id"] == id)
                .context("unknown video preset")?;
            let args: Vec<String> = serde_json::from_value(preset["args"].clone())?;
            command.extend(args);
        }
    }
    for (key, flag) in [
        ("embed_metadata", "--embed-metadata"),
        ("embed_thumbnail", "--embed-thumbnail"),
        ("embed_chapters", "--embed-chapters"),
    ] {
        if b(key) && thumbnail.is_none() {
            command.push(flag.into());
        }
    }
    if b("write_extra") {
        command.extend(["--write-description".into(), "--write-thumbnail".into()]);
    }
    match s("sponsorblock").as_str() {
        "remove" | "sponsors" | "sponsors_promo" => command.extend([
            "--sponsorblock-remove".into(),
            "sponsor,selfpromo,interaction".into(),
        ]),
        "mark" => command.extend(["--sponsorblock-mark".into(), "all".into()]),
        _ => {}
    }
    if b("subs_enabled") {
        command.extend([
            "--write-subs".into(),
            "--sub-langs".into(),
            fallback("sub_langs"),
        ]);
        if b("auto_subs") {
            command.push("--write-auto-subs".into());
        }
        if b("embed_subs") {
            command.push("--embed-subs".into());
        }
    }
    let directory = config::user_path(if out_dir.is_empty() {
        cfg["download_dir"]
            .as_str()
            .context("download_dir must be a string")?
    } else {
        out_dir
    });
    let template = if s("output_template").is_empty() {
        if playlist {
            "%(playlist_title)s/%(playlist_index)03d - %(title)s.%(ext)s".into()
        } else {
            "%(title)s.%(ext)s".into()
        }
    } else {
        s("output_template")
    };
    ensure!(
        !Path::new(&template).is_absolute(),
        "output template must be relative to output directory"
    );
    command.extend([
        "-o".into(),
        media::text_path(&directory.join(template)),
        "--".into(),
        url.into(),
    ]);
    Ok(Plan {
        commands: vec![command],
        target: media::text_path(&directory),
        replacement: None,
        external: if external {
            Some(media::ExternalTranscode {
                preset: preset_id,
                fields: options.clone(),
                config: cfg.clone(),
            })
        } else {
            None
        },
    })
}
