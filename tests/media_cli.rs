use serde_json::{Value, json};
use std::{
    path::{Path, PathBuf},
    process::{Command, Output},
};
struct Fixture {
    directory: tempfile::TempDir,
    input: PathBuf,
}
impl Fixture {
    fn new() -> Self {
        let directory = tempfile::tempdir().unwrap();
        let input = directory.path().join("sample video.mp4");
        let output = Command::new("ffmpeg")
            .args([
                "-v",
                "error",
                "-f",
                "lavfi",
                "-i",
                "testsrc2=size=160x90:rate=25",
                "-f",
                "lavfi",
                "-i",
                "sine=frequency=440:sample_rate=48000",
                "-t",
                "3",
                "-c:v",
                "libx264",
                "-pix_fmt",
                "yuv420p",
                "-c:a",
                "aac",
                "-shortest",
            ])
            .arg(&input)
            .output()
            .expect("FFmpeg is required for integration tests");
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        Self { directory, input }
    }
    fn run(&self, args: &[&str]) -> Output {
        Command::new(env!("CARGO_BIN_EXE_mediacli"))
            .args(args)
            .env("XDG_CONFIG_HOME", self.directory.path().join("config"))
            .env("APPDATA", self.directory.path().join("config"))
            .env_remove("MEDIACLI_SERVER")
            .env_remove("MEDIACLI_TOKEN")
            .output()
            .unwrap()
    }
    fn ok(&self, args: &[&str]) -> Value {
        let output = self.run(args);
        assert!(
            output.status.success(),
            "{args:?}\n{}",
            String::from_utf8_lossy(&output.stderr)
        );
        serde_json::from_slice(&output.stdout).unwrap_or(Value::Null)
    }
    fn path(&self, name: &str) -> PathBuf {
        self.directory.path().join(name)
    }
}
fn text(path: &Path) -> &str {
    path.to_str().unwrap()
}
fn probe(path: &Path) -> Value {
    let output = Command::new("ffprobe")
        .args([
            "-v",
            "error",
            "-show_format",
            "-show_streams",
            "-of",
            "json",
        ])
        .arg(path)
        .output()
        .unwrap();
    assert!(output.status.success());
    serde_json::from_slice(&output.stdout).unwrap()
}
#[test]
fn cli_encodes_trims_filters_and_extracts_audio() {
    let f = Fixture::new();
    let trimmed = f.path("trimmed.mp4");
    f.ok(&[
        "trim",
        text(&f.input),
        "--start",
        "1",
        "--end",
        "2",
        "-o",
        text(&trimmed),
        "--json",
    ]);
    let info = probe(&trimmed);
    let duration = info["format"]["duration"]
        .as_str()
        .unwrap()
        .parse::<f64>()
        .unwrap();
    assert!((duration - 1.0).abs() < 0.15);
    let scaled = f.path("scaled.mp4");
    f.ok(&[
        "scale",
        text(&f.input),
        "--width",
        "80",
        "--height",
        "46",
        "-o",
        text(&scaled),
    ]);
    let info = probe(&scaled);
    assert_eq!(info["streams"][0]["width"], 80);
    let audio = f.path("audio.flac");
    f.ok(&[
        "convert",
        text(&f.input),
        "--preset",
        "audio_flac",
        "-o",
        text(&audio),
    ]);
    assert_eq!(probe(&audio)["streams"][0]["codec_type"], "audio");
    let volume = f.path("volume.mp4");
    f.ok(&[
        "audio",
        text(&f.input),
        "--volume",
        "0.5",
        "-o",
        text(&volume),
    ]);
    let waveform = f.ok(&["waveform", text(&f.input), "--columns", "16", "--json"]);
    assert_eq!(waveform.as_array().unwrap().len(), 16);
}
#[test]
fn projects_round_trip_and_invalid_jobs_preserve_sources() {
    let f = Fixture::new();
    let original = std::fs::read(&f.input).unwrap();
    let project = f.path("project.json");
    f.ok(&[
        "pipeline",
        text(&f.input),
        "--op",
        "trim={\"start\":\"0.5\",\"end\":\"1.5\",\"mode\":\"reencode\"}",
        "--save-project",
        text(&project),
    ]);
    let result = f.path("project-output.mp4");
    f.ok(&["pipeline", "--project", text(&project), "-o", text(&result)]);
    assert!(result.is_file());
    let output = f.run(&[
        "convert",
        text(&f.input),
        "-o",
        text(&f.input),
        "--overwrite",
    ]);
    assert!(!output.status.success());
    assert_eq!(std::fs::read(&f.input).unwrap(), original);
    let output = f.run(&["trim", text(&f.input), "--start", "2", "--end", "1"]);
    assert!(!output.status.success());
    let output = f.run(&[
        "convert",
        text(&f.input),
        "--vcodec",
        "copy",
        "--op",
        "scale={\"width\":80}",
        "--dry-run",
    ]);
    assert!(!output.status.success());
}
#[test]
fn aspect_concat_ranges_size_and_metadata_work() {
    let f = Fixture::new();
    for mode in ["crop", "letterbox", "blur", "stretch"] {
        let output = f.path(&format!("{mode}.mp4"));
        f.ok(&[
            "aspect",
            text(&f.input),
            "--ratio",
            "1:1",
            "--width",
            "90",
            "--height",
            "90",
            "--mode",
            mode,
            "-o",
            text(&output),
        ]);
        assert_eq!(probe(&output)["streams"][0]["width"], 90);
    }
    let joined = f.path("joined.mp4");
    f.ok(&[
        "concat",
        text(&f.input),
        text(&f.input),
        "--mode",
        "filter",
        "-o",
        text(&joined),
    ]);
    let seconds = probe(&joined)["format"]["duration"]
        .as_str()
        .unwrap()
        .parse::<f64>()
        .unwrap();
    assert!((seconds - 6.0).abs() < 0.2);
    let ranges = f.path("ranges.mp4");
    f.ok(&[
        "ranges",
        text(&f.input),
        "--range",
        "0..0.5",
        "--range",
        "1..1.5",
        "--join",
        "-o",
        text(&ranges),
    ]);
    assert!(ranges.is_file());
    let small = f.path("small.mp4");
    f.ok(&[
        "size",
        text(&f.input),
        "--size-mb",
        "0.2",
        "--two-pass",
        "-o",
        text(&small),
    ]);
    assert!(small.is_file());
    let tagged = f.path("tagged.mp4");
    f.ok(&[
        "metadata",
        text(&f.input),
        "--title",
        "Rust test",
        "-o",
        text(&tagged),
    ]);
    assert_eq!(probe(&tagged)["format"]["tags"]["title"], "Rust test");
}
#[test]
fn dry_run_and_batch_validation_do_not_start_media_tools() {
    let f = Fixture::new();
    let output = f.path("dry-run.mp4");
    let plan = f.ok(&[
        "convert",
        text(&f.input),
        "-o",
        text(&output),
        "--dry-run",
        "--json",
    ]);
    assert!(plan["commands"].is_array());
    assert!(!output.exists());
    let destination = f.path("batch");
    let report = f.ok(&[
        "batch",
        text(&f.input),
        "--op",
        "transcode={\"vcodec\":\"libx264\",\"acodec\":\"aac\",\"container\":\"mp4\"}",
        "--output-dir",
        text(&destination),
        "--dry-run",
        "--json",
    ]);
    assert_eq!(report["valid_count"], json!(1));
    assert!(!destination.exists());
    assert_eq!(f.run(&["trim", "--unknown"]).status.code(), Some(2));
}
