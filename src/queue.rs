use crate::{
    config,
    media::{self, Plan},
    storage,
};
use anyhow::{Context, Result, ensure};
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};
use std::{
    collections::BTreeMap,
    path::PathBuf,
    sync::{
        Arc, Mutex,
        atomic::{AtomicUsize, Ordering},
    },
    time::Duration,
};
use tokio::{
    io::{AsyncRead, AsyncReadExt},
    process::Command,
    sync::Notify,
};
use tokio_util::sync::CancellationToken;

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Task {
    pub id: u64,
    pub title: String,
    pub source: String,
    pub target: String,
    pub status: String,
    pub stage: String,
    pub progress: f64,
    pub exit_code: i32,
    pub finalized: bool,
    pub started_at: String,
    pub finished_at: String,
    pub log_tail: Vec<String>,
    pub group_id: String,
    pub chain_index: usize,
    pub chain_total: usize,
    pub plan: Plan,
    #[serde(default)]
    pub output_files: Vec<String>,
    #[serde(skip)]
    pub cancellation: CancellationToken,
}
impl Task {
    pub fn active(&self) -> bool {
        self.status == "queued" || self.status == "running"
    }
    pub fn snapshot(&self) -> Value {
        json!({"id":self.id,"title":self.title,"source":self.source,"target":self.target,"status":self.status,"stage":self.stage,"progress":self.progress,"exit_code":self.exit_code,"finalized":self.finalized,"started_at":self.started_at,"finished_at":self.finished_at,"log_tail":self.log_tail.iter().rev().take(100).cloned().collect::<Vec<_>>().into_iter().rev().collect::<Vec<_>>(),"group_id":self.group_id,"chain_index":self.chain_index,"chain_total":self.chain_total})
    }
}
struct State {
    tasks: BTreeMap<u64, Task>,
    next_id: u64,
    persistence: Option<PathBuf>,
    persistence_error: Option<String>,
}
#[derive(Clone)]
pub struct Queue {
    state: Arc<Mutex<State>>,
    maximum: Arc<AtomicUsize>,
    notify: Arc<Notify>,
}
impl Queue {
    pub fn new(maximum: usize, persistence: Option<PathBuf>) -> Result<Self> {
        let mut state = State {
            tasks: BTreeMap::new(),
            next_id: 1,
            persistence,
            persistence_error: None,
        };
        if let Some(path) = &state.persistence {
            let records = storage::read_json(path, json!([]))?;
            let array = records.as_array().context("invalid queue journal")?;
            if array.iter().any(|r| r.get("Task").is_some()) {
                let backup = path.with_extension("go-backup.json");
                if !backup.exists() {
                    storage::write_json(&backup, &records)?;
                }
            }
            for record in array {
                let mut task: Task = if record.get("Task").is_some() {
                    import_legacy(record, array)?
                } else {
                    serde_json::from_value(record.clone())?
                };
                ensure!(
                    task.id > 0
                        && !state.tasks.contains_key(&task.id)
                        && !task.plan.commands.is_empty(),
                    "invalid queue record"
                );
                if task.active() {
                    task.status = "interrupted".into();
                    task.stage = "Interrupted; retry from queue".into();
                    task.finalized = true;
                }
                state.next_id = state.next_id.max(task.id + 1);
                state.tasks.insert(task.id, task);
            }
        }
        Ok(Self {
            state: Arc::new(Mutex::new(state)),
            maximum: Arc::new(AtomicUsize::new(maximum.max(1))),
            notify: Arc::new(Notify::new()),
        })
    }
    fn persist(state: &mut State) {
        if let Some(path) = &state.persistence {
            let tasks: Vec<_> = state.tasks.values().collect();
            if let Err(error) = storage::write_json(path, &tasks) {
                state.persistence_error = Some(error.to_string());
            } else {
                state.persistence_error = None;
            }
        }
    }
    pub fn persistence_error(&self) -> Option<String> {
        self.state.lock().unwrap().persistence_error.clone()
    }
    pub fn set_maximum(&self, value: usize) {
        self.maximum.store(value.clamp(1, 32), Ordering::Relaxed);
        self.notify.notify_waiters();
    }
    pub fn list(&self) -> Vec<Task> {
        self.state.lock().unwrap().tasks.values().cloned().collect()
    }
    pub fn get(&self, id: u64) -> Option<Task> {
        self.state.lock().unwrap().tasks.get(&id).cloned()
    }
    fn change(&self, id: u64, update: impl FnOnce(&mut Task)) {
        let mut state = self.state.lock().unwrap();
        if let Some(task) = state.tasks.get_mut(&id) {
            update(task);
        }
        Self::persist(&mut state);
    }
    pub fn enqueue(&self, plan: Plan, title: &str, source: &str, group: &str) -> Result<u64> {
        ensure!(
            !plan.commands.is_empty() && plan.commands.iter().all(|c| !c.is_empty()),
            "empty command plan"
        );
        let target = PathBuf::from(&plan.target);
        if let Some(parent) = target.parent().filter(|p| !p.as_os_str().is_empty()) {
            std::fs::create_dir_all(parent)?;
        }
        if plan.commands[0][0] == "yt-dlp" {
            std::fs::create_dir_all(&plan.target)?;
        }
        let id = {
            let mut state = self.state.lock().unwrap();
            let id = state.next_id;
            state.next_id += 1;
            let total = plan.commands.len();
            state.tasks.insert(
                id,
                Task {
                    id,
                    title: title.into(),
                    source: source.into(),
                    target: plan.target.clone(),
                    status: "queued".into(),
                    stage: "Queued".into(),
                    progress: 0.0,
                    exit_code: 0,
                    finalized: false,
                    started_at: String::new(),
                    finished_at: String::new(),
                    log_tail: Vec::new(),
                    group_id: group.into(),
                    chain_index: 0,
                    chain_total: total,
                    plan,
                    output_files: Vec::new(),
                    cancellation: CancellationToken::new(),
                },
            );
            Self::persist(&mut state);
            id
        };
        let queue = self.clone();
        tokio::spawn(async move {
            queue.worker(id).await;
        });
        Ok(id)
    }
    pub fn cancel(&self, id: u64) -> bool {
        let mut state = self.state.lock().unwrap();
        let Some(task) = state.tasks.get_mut(&id) else {
            return false;
        };
        if !task.active() {
            return false;
        }
        task.cancellation.cancel();
        if task.status == "queued" {
            task.status = "cancelled".into();
            task.stage = "Cancelled".into();
            task.exit_code = 130;
            task.finalized = true;
            task.finished_at = now();
        }
        Self::persist(&mut state);
        self.notify.notify_waiters();
        true
    }
    pub fn cancel_all(&self) {
        for task in self.list() {
            self.cancel(task.id);
        }
    }
    pub fn cancel_group(&self, group: &str) -> usize {
        self.list()
            .iter()
            .filter(|t| t.group_id == group && self.cancel(t.id))
            .count()
    }
    pub fn retry(&self, id: u64) -> Result<u64> {
        let task = self.get(id).context("task not found")?;
        ensure!(!task.active(), "task is active");
        self.replay(task.plan.clone(), &task.title, &task.source)
    }
    pub fn replay(&self, mut plan: Plan, title: &str, source: &str) -> Result<u64> {
        ensure!(
            !plan.commands.is_empty() && plan.commands.iter().all(|c| !c.is_empty()),
            "no replayable command saved"
        );

        if let Some(replacement) = plan.replacement.as_mut() {
            let previous = replacement.temporary.clone();
            let ext = PathBuf::from(&previous)
                .extension()
                .unwrap_or_default()
                .to_string_lossy()
                .into_owned();
            replacement.temporary = media::unique_output(
                &previous,
                &format!("_retry_{}", media::timestamp()),
                &ext,
                "",
            );
            for command in &mut plan.commands {
                for argument in command {
                    if argument == &previous {
                        *argument = replacement.temporary.clone();
                    }
                }
            }
        } else if let Some(last) = plan
            .commands
            .last_mut()
            .filter(|c| c.iter().any(|a| a == "-i"))
        {
            let old = last.last().context("empty command")?.clone();
            if PathBuf::from(&old).exists() {
                let ext = PathBuf::from(&old)
                    .extension()
                    .unwrap_or_default()
                    .to_string_lossy()
                    .into_owned();
                plan.target = media::unique_output(&old, "_retry", &ext, "");
                *last.last_mut().unwrap() = plan.target.clone();
            }
        }
        for command in &mut plan.commands {
            if command.iter().any(|a| a == "-i")
                && command
                    .last()
                    .is_some_and(|a| a != "-" && a != "/dev/null" && !a.eq_ignore_ascii_case("NUL"))
            {
                let inputs: Vec<String> = command
                    .windows(2)
                    .filter(|pair| pair[0] == "-i")
                    .map(|pair| pair[1].clone())
                    .collect();
                media::protect_output(command.last().unwrap(), &inputs, false)?;
                for argument in command.iter_mut() {
                    if argument == "-y" {
                        *argument = "-n".into();
                    }
                }
            }
        }
        let mut logs = BTreeMap::new();
        for command in &mut plan.commands {
            for index in 0..command.len().saturating_sub(1) {
                if command[index] == "-passlogfile" {
                    let old = command[index + 1].clone();
                    let replacement = logs.entry(old).or_insert_with(|| {
                        media::text_path(
                            &std::env::temp_dir()
                                .join(format!("mediacli-pass-{}", media::timestamp())),
                        )
                    });
                    command[index + 1] = replacement.clone();
                }
            }
        }
        self.enqueue(
            plan,
            title,
            source,
            &format!("retry_{}", media::timestamp()),
        )
    }
    pub async fn wait(&self, id: u64) -> Result<Task> {
        loop {
            let task = self.get(id).context("task not found")?;
            if task.finalized {
                return Ok(task);
            }
            tokio::time::sleep(Duration::from_millis(30)).await;
        }
    }
    pub async fn shutdown(&self) {
        self.cancel_all();
        for _ in 0..250 {
            if self.list().iter().all(|t| t.finalized) {
                return;
            }
            tokio::time::sleep(Duration::from_millis(20)).await;
        }
    }
    async fn worker(&self, id: u64) {
        loop {
            let notification = self.notify.notified();
            let admitted = {
                let mut state = self.state.lock().unwrap();
                let running = state
                    .tasks
                    .values()
                    .filter(|t| t.status == "running")
                    .count();
                let first = state
                    .tasks
                    .values()
                    .find(|t| t.status == "queued")
                    .map(|t| t.id);
                let Some(task) = state.tasks.get_mut(&id) else {
                    return;
                };
                if task.cancellation.is_cancelled() {
                    return;
                }
                if running < self.maximum.load(Ordering::Relaxed) && first == Some(id) {
                    task.status = "running".into();
                    task.started_at = now();
                    task.stage = "Starting".into();
                    Self::persist(&mut state);
                    true
                } else {
                    false
                }
            };
            if admitted {
                self.notify.notify_waiters();
                break;
            }
            tokio::select! {_ = notification=>{},_ = tokio::time::sleep(Duration::from_millis(100))=>{}}
        }
        let task = self.get(id).unwrap();
        let mut code = 0;
        let mut failure = None;
        for (index, command) in task.plan.commands.iter().enumerate() {
            if task.cancellation.is_cancelled() {
                code = 130;
                break;
            }
            self.change(id, |t| {
                t.chain_index = index;
                t.stage = format!("Pass {}/{}", index + 1, t.chain_total);
            });
            match self.execute(id, command, &task.cancellation).await {
                Ok(exit) => code = exit,
                Err(error) => {
                    code = 1;
                    failure = Some(error.to_string());
                }
            }
            if code != 0 {
                break;
            }
        }
        if code == 0
            && let Some(external) = &task.plan.external
        {
            let files = self.get(id).unwrap().output_files;
            if files.is_empty() {
                code = 1;
                failure = Some("yt-dlp did not report an output file".into());
            }
            for file in files {
                if task.cancellation.is_cancelled() {
                    code = 130;
                    break;
                }
                self.change(id, |task| task.stage = "External FFmpeg transcode".into());
                let plan = if external.preset == "custom" {
                    let extension = config::string(&external.fields, "custom_ext");
                    let extension = if extension.is_empty() {
                        "mp4"
                    } else {
                        &extension
                    };
                    let flags = config::string(&external.fields, "custom_flags");
                    let target = media::unique_output(&file, "_converted", extension, "");
                    let mut command = vec![
                        config::tool(&external.config, "ffmpeg"),
                        "-nostdin".into(),
                        "-n".into(),
                        "-i".into(),
                        file.clone(),
                    ];
                    match shell_words::split(&flags) {
                        Ok(flags) => {
                            command.extend(flags);
                            command.push(target.clone());
                            Ok(Plan {
                                commands: vec![command],
                                target,
                                ..Default::default()
                            })
                        }
                        Err(error) => Err(anyhow::Error::new(error)),
                    }
                } else {
                    media::conversion_plan(&external.preset, &file, "", &external.config)
                };
                match plan {
                    Ok(plan) => {
                        for command in &plan.commands {
                            match self.execute(id, command, &task.cancellation).await {
                                Ok(exit) => code = exit,
                                Err(error) => {
                                    code = 1;
                                    failure = Some(error.to_string());
                                }
                            }
                            if code != 0 {
                                break;
                            }
                        }
                        if let Some(replacement) = &plan.replacement
                            && let Err(error) = replacement.finish(code == 0)
                        {
                            code = 1;
                            failure = Some(error.to_string());
                        }
                        if code == 0 {
                            self.change(id, |task| task.target = plan.target.clone());
                        }
                    }
                    Err(error) => {
                        code = 1;
                        failure = Some(error.to_string());
                    }
                }
                if code != 0 {
                    break;
                }
            }
        }
        if task.cancellation.is_cancelled() {
            code = 130;
        }
        if let Some(replacement) = &task.plan.replacement
            && let Err(error) = replacement.finish(code == 0)
        {
            code = 1;
            failure = Some(error.to_string());
        }
        for command in &task.plan.commands {
            cleanup_pass_logs(command);
        }
        if let Some(message) = failure {
            self.log(id, message);
        }
        let status = if code == 0 {
            "done"
        } else if code == 130 {
            "cancelled"
        } else {
            "failed"
        };
        let entry = json!({"time":chrono::Local::now().format("%Y-%m-%d %H:%M:%S").to_string(),"type":task.title,"source":task.source,"target":task.target,"status":status,"commands":if task.plan.replacement.is_none(){serde_json::to_value(&task.plan.commands).unwrap()}else{json!([])}});
        if let Err(error) = config::add_history(entry) {
            self.log(id, format!("Save history: {error}"));
        }
        self.change(id, |t| {
            t.status = status.into();
            t.stage = status.into();
            t.exit_code = code;
            t.finalized = true;
            t.finished_at = now();
            if code == 0 {
                t.progress = 100.0;
            }
        });
        self.notify.notify_waiters();
    }
    pub fn log(&self, id: u64, line: String) {
        let mut state = self.state.lock().unwrap();
        if let Some(task) = state.tasks.get_mut(&id) {
            if let Some(path) = line
                .strip_prefix("MEDIACLI_FILE=")
                .and_then(|text| serde_json::from_str::<String>(text).ok())
                && !task.output_files.contains(&path)
            {
                task.output_files.push(path);
            }
            if let Some(progress) = progress(&line) {
                task.progress = progress;
            }
            task.log_tail.push(line);
            if task.log_tail.len() > 10000 {
                task.log_tail.drain(..1000);
            }
        }
    }
    async fn execute(
        &self,
        id: u64,
        arguments: &[String],
        cancel: &CancellationToken,
    ) -> Result<i32> {
        let mut command = Command::new(&arguments[0]);
        command
            .args(&arguments[1..])
            .stdin(std::process::Stdio::null())
            .stdout(std::process::Stdio::piped())
            .stderr(std::process::Stdio::piped())
            .kill_on_drop(true);
        #[cfg(unix)]
        command.process_group(0);
        let mut child = command
            .spawn()
            .with_context(|| format!("start {}", arguments[0]))?;
        let stdout = child.stdout.take().unwrap();
        let stderr = child.stderr.take().unwrap();
        let stdout_queue = self.clone();
        let stderr_queue = self.clone();
        let stdout_task = tokio::spawn(async move {
            stdout_queue.read_stream(id, stdout).await;
        });
        let stderr_task = tokio::spawn(async move {
            stderr_queue.read_stream(id, stderr).await;
        });
        let code = tokio::select! {
            result=child.wait()=>result?.code().unwrap_or(1),
            _ = cancel.cancelled()=>{
                #[cfg(unix)]
                if let Some(pid)=child.id(){ unsafe { libc::kill(-(pid as i32),libc::SIGKILL); } }
                #[cfg(windows)]
                if let Some(pid)=child.id(){ let _=Command::new("taskkill").args(["/PID",&pid.to_string(),"/T","/F"]).output().await; }
                let _=child.kill().await;130
            }
        };
        // Both readers must finish before finalization so no completion or file events are lost.
        let _ = stdout_task.await;
        let _ = stderr_task.await;
        Ok(code)
    }
    async fn read_stream(&self, id: u64, mut stream: impl AsyncRead + Unpin) {
        let mut buffer = [0; 8192];
        let mut line = Vec::new();
        loop {
            match stream.read(&mut buffer).await {
                Ok(0) => break,
                Ok(length) => {
                    for byte in &buffer[..length] {
                        if *byte == b'\n' || *byte == b'\r' {
                            if !line.is_empty() {
                                self.log(id, String::from_utf8_lossy(&line).into_owned());
                                line.clear();
                            }
                        } else {
                            line.push(*byte);
                            if line.len() >= 65536 {
                                self.log(id, String::from_utf8_lossy(&line).into_owned());
                                line.clear();
                            }
                        }
                    }
                }
                Err(error) => {
                    self.log(id, format!("read output: {error}"));
                    break;
                }
            }
        }
        if !line.is_empty() {
            self.log(id, String::from_utf8_lossy(&line).into_owned());
        }
    }
}
pub fn now() -> String {
    chrono::Utc::now().to_rfc3339()
}
pub fn progress(line: &str) -> Option<f64> {
    let percent = line.find('%')?;
    let prefix = &line[..percent];
    let raw = prefix
        .rsplit(|c: char| !(c.is_ascii_digit() || c == '.'))
        .next()?;
    let value: f64 = raw.parse().ok()?;
    if value.is_finite() {
        Some(value.clamp(0.0, 100.0))
    } else {
        None
    }
}

fn import_legacy(record: &Value, records: &[Value]) -> Result<Task> {
    let old = &record["Task"];
    let id = old["ID"].as_u64().context("invalid legacy task ID")?;
    let mut root = record;
    let mut visited = std::collections::BTreeSet::new();
    while let Some(previous) = root["Previous"].as_u64().filter(|id| *id != 0) {
        ensure!(visited.insert(previous), "cyclic legacy queue dependency");
        root = records
            .iter()
            .find(|r| r["Task"]["ID"].as_u64() == Some(previous))
            .context("missing legacy queue dependency")?;
    }
    let mut chain = vec![root];
    loop {
        let last = chain.last().unwrap()["Task"]["ID"].as_u64().unwrap();
        let next = records
            .iter()
            .find(|r| r["Previous"].as_u64() == Some(last));
        let Some(next) = next else { break };
        ensure!(chain.len() < records.len(), "cyclic queue");
        chain.push(next);
    }
    let commands = chain
        .iter()
        .map(|r| serde_json::from_value::<Vec<String>>(r["Task"]["Cmd"].clone()))
        .collect::<std::result::Result<Vec<_>, _>>()?;
    let mut replacement = None;
    for item in &chain {
        let legacy = &item["Task"]["Replacement"];
        if !legacy.is_null() {
            replacement = Some(media::Replacement {
                input: config::string(legacy, "Input"),
                temporary: config::string(legacy, "Temporary"),
                final_path: config::string(legacy, "Final"),
            });
        } else {
            ensure!(
                !config::boolean(item, "HasHook"),
                "legacy queue has an unrecoverable completion hook"
            );
        }
    }
    let status = config::string(old, "Status");
    Ok(Task {
        id,
        title: config::string(old, "Title"),
        source: config::string(old, "Source"),
        target: config::string(old, "Target"),
        status,
        stage: config::string(old, "Stage"),
        progress: config::number(old, "Progress", 0.0),
        exit_code: config::number(old, "ExitCode", 0.0) as i32,
        finalized: true,
        started_at: config::string(old, "StartedAt"),
        finished_at: config::string(old, "FinishedAt"),
        log_tail: Vec::new(),
        group_id: config::string(old, "group_id"),
        chain_index: 0,
        chain_total: commands.len(),
        plan: Plan {
            commands,
            target: config::string(old, "Target"),
            replacement,
            external: None,
        },
        output_files: Vec::new(),
        cancellation: CancellationToken::new(),
    })
}

fn cleanup_pass_logs(command: &[String]) {
    let prefix = command
        .windows(2)
        .find(|pair| pair[0] == "-passlogfile")
        .map(|pair| &pair[1]);
    if let Some(prefix) = prefix
        && PathBuf::from(prefix)
            .file_name()
            .is_some_and(|name| name.to_string_lossy().starts_with("mediacli-pass-"))
    {
        for suffix in ["-0.log", "-0.log.mbtree"] {
            let _ = std::fs::remove_file(format!("{prefix}{suffix}"));
        }
    }
}
