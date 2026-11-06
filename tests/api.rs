#![cfg(unix)]
use serde_json::{Value, json};
use std::{
    io::{BufRead, BufReader},
    process::{Child, Command, Stdio},
    time::{Duration, Instant},
};
struct Daemon {
    child: Child,
    base: String,
    directory: tempfile::TempDir,
    client: reqwest::blocking::Client,
}
impl Drop for Daemon {
    fn drop(&mut self) {
        unsafe {
            libc::kill(self.child.id() as i32, libc::SIGTERM);
        }
        let _ = self.child.wait();
    }
}
impl Daemon {
    fn start() -> Self {
        let directory = tempfile::tempdir().unwrap();
        let mut child = Command::new(env!("CARGO_BIN_EXE_mediacli"))
            .args(["daemon", "--port", "0", "--persist"])
            .env("XDG_CONFIG_HOME", directory.path())
            .env("MEDIACLI_TOKEN", "test-token")
            .stdout(Stdio::piped())
            .stderr(Stdio::inherit())
            .spawn()
            .unwrap();
        let mut reader = BufReader::new(child.stdout.take().unwrap());
        let mut line = String::new();
        reader.read_line(&mut line).unwrap();
        let port = line
            .trim()
            .strip_prefix("MEDIACLI_DAEMON_PORT=")
            .unwrap()
            .parse::<u16>()
            .unwrap();
        Self {
            child,
            base: format!("http://127.0.0.1:{port}"),
            directory,
            client: reqwest::blocking::Client::builder()
                .no_proxy()
                .timeout(Duration::from_secs(10))
                .build()
                .unwrap(),
        }
    }
    fn request(&self, method: &str, path: &str, body: Value) -> Value {
        let response = self
            .client
            .request(method.parse().unwrap(), format!("{}{path}", self.base))
            .bearer_auth("test-token")
            .json(&body)
            .send()
            .unwrap();
        let status = response.status();
        let value: Value = response.json().unwrap();
        assert!(status.is_success(), "{method} {path}: {value}");
        value
    }
    fn job(&self, commands: Value) -> u64 {
        self.request(
            "POST",
            "/api/jobs",
            json!({"plan":{"commands":commands,"target":""},"title":"Test","source":"fixture"}),
        )["task_id"]
            .as_u64()
            .unwrap()
    }
    fn wait(&self, id: u64) -> Value {
        let start = Instant::now();
        loop {
            let task = self.request("GET", &format!("/api/tasks/{id}"), json!({}));
            if task["finalized"] == true {
                return task;
            }
            assert!(
                start.elapsed() < Duration::from_secs(5),
                "task hung: {task}"
            );
            std::thread::sleep(Duration::from_millis(20));
        }
    }
}
#[test]
fn authenticated_api_matches_electron_contract() {
    let daemon = Daemon::start();
    let response = daemon
        .client
        .get(format!("{}/api/status", daemon.base))
        .send()
        .unwrap();
    assert_eq!(response.status(), 401);
    let status = daemon.request("GET", "/api/status", json!({}));
    assert_eq!(status["version"], "0.1.2");
    assert!(status["dependencies"].is_array());
    for (endpoint, key) in [
        ("/api/presets", "video_presets"),
        ("/api/convert/presets", "convert_presets"),
        ("/api/meta", "themes"),
        ("/api/tasks", "tasks"),
        ("/api/history", "history"),
        ("/api/pipeline/presets", "presets"),
    ] {
        assert!(
            daemon.request("GET", endpoint, json!({}))[key].is_array(),
            "{endpoint}"
        );
    }
    let config = daemon.request(
        "PUT",
        "/api/config",
        json!({"bg_queue_max":1,"language":"ru"}),
    );
    assert_eq!(config["language"], "ru");
    assert_eq!(config["bg_queue_max"], 1);
    let response = daemon
        .client
        .put(format!("{}/api/config", daemon.base))
        .bearer_auth("test-token")
        .json(&json!({"bg_queue_max":0}))
        .send()
        .unwrap();
    assert_eq!(response.status(), 400);
    let job = daemon.job(json!([
        ["sh", "-c", "printf 'first\\n'; sleep 0.1"],
        ["sh", "-c", "printf 'second\\n'"]
    ]));
    let result = daemon.wait(job);
    assert_eq!(result["status"], "done");
    let logs = daemon.request("GET", &format!("/api/tasks/{job}/logs"), json!({}));
    assert_eq!(logs["logs"], json!(["first", "second"]));
    let response = daemon
        .client
        .get(format!(
            "{}/api/tasks/{job}/events?token=test-token",
            daemon.base
        ))
        .send()
        .unwrap();
    assert!(
        response.headers()["content-type"]
            .to_str()
            .unwrap()
            .starts_with("text/event-stream")
    );
    assert!(response.text().unwrap().contains("\"finalized\":true"));
}
#[test]
fn failed_chain_stops_and_cancellation_kills_child_processes() {
    let daemon = Daemon::start();
    let marker = daemon.directory.path().join("should-not-exist");
    let id = daemon.job(json!([["sh", "-c", "exit 17"], ["touch", marker]]));
    let task = daemon.wait(id);
    assert_eq!(task["status"], "failed");
    assert!(!marker.exists());
    let id = daemon.job(json!([["sh", "-c", "sleep 30 & wait"]]));
    std::thread::sleep(Duration::from_millis(150));
    let start = Instant::now();
    daemon.request("POST", &format!("/api/tasks/{id}/cancel"), json!({}));
    let task = daemon.wait(id);
    assert_eq!(task["status"], "cancelled");
    assert!(start.elapsed() < Duration::from_secs(2));
    let retried = daemon.request("POST", &format!("/api/tasks/{id}/retry"), json!({}));
    let id = retried["task_ids"][0].as_u64().unwrap();
    daemon.request("POST", &format!("/api/tasks/{id}/cancel"), json!({}));
    daemon.wait(id);
}
#[test]
fn media_http_ranges_and_symlink_boundaries() {
    let daemon = Daemon::start();
    let library = daemon.directory.path().join("downloads");
    std::fs::create_dir(&library).unwrap();
    let media = library.join("test.mp4");
    std::fs::write(&media, b"0123456789").unwrap();
    let outside = daemon.directory.path().join("private.mp4");
    std::fs::write(&outside, b"private").unwrap();
    std::os::unix::fs::symlink(&outside, library.join("escape.mp4")).unwrap();
    daemon.request("PUT", "/api/config", json!({"download_dir":library}));
    let response = daemon
        .client
        .get(format!(
            "{}/api/library/file?name=test.mp4&token=test-token",
            daemon.base
        ))
        .header("Range", "bytes=2-5")
        .send()
        .unwrap();
    assert_eq!(response.status(), 206);
    assert_eq!(response.headers()["content-range"], "bytes 2-5/10");
    assert_eq!(response.text().unwrap(), "2345");
    let response = daemon
        .client
        .get(format!(
            "{}/api/library/file?name=test.mp4&token=test-token",
            daemon.base
        ))
        .header("Range", "bytes=-3")
        .send()
        .unwrap();
    assert_eq!(response.text().unwrap(), "789");
    let response = daemon
        .client
        .get(format!(
            "{}/api/library/file?name=escape.mp4&token=test-token",
            daemon.base
        ))
        .send()
        .unwrap();
    assert_eq!(response.status(), 400);
}
#[test]
fn legacy_queue_is_imported_as_interrupted_with_a_backup() {
    let directory = tempfile::tempdir().unwrap();
    let queue = directory.path().join("queue.json");
    let records = json!([{"Task":{"ID":1,"Cmd":["sh","-c","printf first"],"Title":"Pass","Source":"input","Target":"output","Status":"running","Stage":"Encoding","Progress":30,"ExitCode":0},"Previous":0,"HasHook":false},{"Task":{"ID":2,"Cmd":["sh","-c","printf second"],"Title":"Pass","Source":"input","Target":"output","Status":"queued","Stage":"Queued","Progress":0,"ExitCode":0},"Previous":1,"HasHook":false}]);
    std::fs::write(&queue, serde_json::to_vec(&records).unwrap()).unwrap();
    let manager = mediacli::queue::Queue::new(1, Some(queue.clone())).unwrap();
    let tasks = manager.list();
    assert_eq!(tasks.len(), 2);
    assert!(
        tasks
            .iter()
            .all(|task| task.status == "interrupted" && task.finalized)
    );
    assert_eq!(tasks[1].plan.commands.len(), 2);
    assert!(queue.with_extension("go-backup.json").is_file());
}
