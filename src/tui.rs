use crate::{
    config::{self, number, string},
    download,
    media::{self, Pipeline},
    queue::Queue,
    storage,
};
use anyhow::{Context, Result, ensure};
use crossterm::{
    event::{self, Event, KeyCode, KeyEvent, KeyEventKind, KeyModifiers},
    execute,
    terminal::{self, EnterAlternateScreen, LeaveAlternateScreen},
};
use ratatui::{
    Terminal,
    backend::CrosstermBackend,
    layout::{Constraint, Direction, Layout, Rect},
    style::{Color, Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Clear, List, ListItem, ListState, Paragraph, Tabs, Wrap},
};
use serde_json::{Value, json};
use std::{
    io::{self, IsTerminal},
    path::PathBuf,
    time::Duration,
};

struct TerminalGuard;
impl Drop for TerminalGuard {
    fn drop(&mut self) {
        let _ = terminal::disable_raw_mode();
        let _ = execute!(io::stdout(), LeaveAlternateScreen, crossterm::cursor::Show);
    }
}
#[derive(Clone)]
struct Action {
    command: &'static str,
    ru: &'static str,
    en: &'static str,
    category: usize,
}
fn actions() -> Vec<Action> {
    vec![
        Action {
            command: "theme",
            ru: "Сменить тему",
            en: "Change theme",
            category: 2,
        },
        Action {
            command: "clear-history",
            ru: "Очистить историю",
            en: "Clear history",
            category: 2,
        },
        Action {
            command: "download",
            ru: "Скачать видео",
            en: "Download video",
            category: 0,
        },
        Action {
            command: "playlist",
            ru: "Скачать плейлист",
            en: "Download playlist",
            category: 0,
        },
        Action {
            command: "audio-download",
            ru: "Скачать аудио",
            en: "Download audio",
            category: 0,
        },
        Action {
            command: "thumbnail",
            ru: "Скачать обложку",
            en: "Download thumbnail",
            category: 0,
        },
        Action {
            command: "convert",
            ru: "Конвертация",
            en: "Convert",
            category: 1,
        },
        Action {
            command: "trim",
            ru: "Обрезка",
            en: "Trim",
            category: 1,
        },
        Action {
            command: "aspect",
            ru: "Соотношение сторон",
            en: "Aspect ratio",
            category: 1,
        },
        Action {
            command: "mux",
            ru: "Добавить дорожки",
            en: "Mux tracks",
            category: 1,
        },
        Action {
            command: "demux",
            ru: "Извлечь дорожку",
            en: "Demux track",
            category: 1,
        },
        Action {
            command: "concat",
            ru: "Склейка",
            en: "Concatenate",
            category: 1,
        },
        Action {
            command: "filter",
            ru: "Фильтры",
            en: "Filters",
            category: 1,
        },
        Action {
            command: "audio",
            ru: "Обработка звука",
            en: "Audio processing",
            category: 1,
        },
        Action {
            command: "subtitle",
            ru: "Субтитры",
            en: "Subtitles",
            category: 1,
        },
        Action {
            command: "metadata",
            ru: "Метаданные",
            en: "Metadata",
            category: 1,
        },
        Action {
            command: "scale",
            ru: "Размер кадра",
            en: "Scale",
            category: 1,
        },
        Action {
            command: "fps",
            ru: "Частота кадров",
            en: "Frame rate",
            category: 1,
        },
        Action {
            command: "stabilize",
            ru: "Стабилизация",
            en: "Stabilize",
            category: 1,
        },
        Action {
            command: "size",
            ru: "Экспорт в заданный размер",
            en: "Export to size",
            category: 1,
        },
        Action {
            command: "ranges",
            ru: "Фрагменты / звуковая волна",
            en: "Ranges / waveform",
            category: 1,
        },
        Action {
            command: "editor",
            ru: "Универсальный редактор",
            en: "Universal editor",
            category: 1,
        },
        Action {
            command: "pipeline",
            ru: "Цепочка операций",
            en: "Pipeline",
            category: 1,
        },
        Action {
            command: "batch",
            ru: "Пакетная обработка",
            en: "Batch processing",
            category: 1,
        },
        Action {
            command: "probe",
            ru: "Информация о файле",
            en: "Inspect file",
            category: 1,
        },
        Action {
            command: "queue",
            ru: "Очередь задач",
            en: "Task queue",
            category: 2,
        },
        Action {
            command: "history",
            ru: "История",
            en: "History",
            category: 2,
        },
        Action {
            command: "settings",
            ru: "Настройки",
            en: "Settings",
            category: 2,
        },
        Action {
            command: "doctor",
            ru: "Проверить зависимости",
            en: "Check dependencies",
            category: 2,
        },
        Action {
            command: "library",
            ru: "Папка загрузок",
            en: "Download library",
            category: 2,
        },
        Action {
            command: "presets",
            ru: "Сохранённые наборы",
            en: "Saved presets",
            category: 2,
        },
    ]
}
#[derive(Clone)]
struct Field {
    key: String,
    kind: String,
    value: String,
}
struct Form {
    title: String,
    action: String,
    fields: Vec<Field>,
    selected: usize,
}
#[derive(Default)]
struct Editor {
    pipeline: Pipeline,
    undo: Vec<Pipeline>,
    redo: Vec<Pipeline>,
    name: String,
}
impl Editor {
    fn path(&self) -> PathBuf {
        config::directory().join(format!("{}_draft.json", self.name))
    }
    fn save(&self) -> Result<()> {
        storage::write_json(&self.path(), &self.pipeline)
    }
    fn capture(&mut self) {
        self.undo.push(self.pipeline.clone());
        if self.undo.len() > 100 {
            self.undo.remove(0);
        }
        self.redo.clear();
    }
    fn move_history(&mut self, redo: bool) -> Result<()> {
        let (source, destination) = if redo {
            (&mut self.redo, &mut self.undo)
        } else {
            (&mut self.undo, &mut self.redo)
        };
        if let Some(previous) = source.pop() {
            destination.push(self.pipeline.clone());
            self.pipeline = previous;
            self.save()?;
        }
        Ok(())
    }
}
struct Ranges {
    input: String,
    cursor: f64,
    start: Option<f64>,
    duration: f64,
    fps: f64,
    ranges: Vec<(f64, f64)>,
    undo: Vec<Vec<(f64, f64)>>,
    redo: Vec<Vec<(f64, f64)>>,
    wave: Vec<f64>,
}
impl Ranges {
    fn path(&self) -> PathBuf {
        let mut hash = 0_u64;
        for byte in self.input.bytes() {
            hash = hash.wrapping_mul(31).wrapping_add(u64::from(byte));
        }
        config::directory().join(format!("ranges_{hash:x}.json"))
    }
    fn save(&self) -> Result<()> {
        storage::write_json(
            &self.path(),
            &json!({"input":self.input,"ranges":self.ranges}),
        )
    }
    fn capture(&mut self) {
        self.undo.push(self.ranges.clone());
        self.redo.clear();
    }
}
struct Picker {
    directory: PathBuf,
    entries: Vec<PathBuf>,
    selected: usize,
}
impl Picker {
    fn read(directory: PathBuf) -> Result<Self> {
        let mut entries = std::fs::read_dir(&directory)?
            .map(|entry| entry.map(|e| e.path()))
            .collect::<std::io::Result<Vec<_>>>()?;
        entries.sort_by_key(|path| {
            (
                !path.is_dir(),
                path.file_name().unwrap_or_default().to_os_string(),
            )
        });
        Ok(Self {
            directory,
            entries,
            selected: 0,
        })
    }
}
struct App {
    cfg: Value,
    queue: Queue,
    tab: usize,
    selected: usize,
    screen: String,
    text: String,
    title: String,
    form: Option<Form>,
    palette: Option<String>,
    palette_selected: usize,
    editor: Editor,
    ranges: Option<Ranges>,
    task: Option<u64>,
    scroll: u16,
    quit: bool,
    confirm: Option<String>,
    picker: Option<Picker>,
    error: Option<String>,
    history_query: String,
}
impl App {
    fn new(cfg: Value, queue: Queue) -> Self {
        Self {
            cfg,
            queue,
            tab: 0,
            selected: 0,
            screen: "home".into(),
            text: String::new(),
            title: String::new(),
            form: None,
            palette: None,
            palette_selected: 0,
            editor: Editor::default(),
            ranges: None,
            task: None,
            scroll: 0,
            quit: false,
            confirm: None,
            picker: None,
            error: None,
            history_query: String::new(),
        }
    }

    fn ru(&self) -> bool {
        string(&self.cfg, "language") == "ru"
    }
    fn tr<'a>(&self, ru: &'a str, en: &'a str) -> &'a str {
        if self.ru() { ru } else { en }
    }
    fn message(&mut self, title: &str, text: String) {
        self.title = title.into();
        self.text = text;
        self.screen = "text".into();
        self.scroll = 0;
    }
    fn field(key: &str, kind: &str, value: Value) -> Field {
        Field {
            key: key.into(),
            kind: kind.into(),
            value: if let Some(text) = value.as_str() {
                text.into()
            } else if value.is_null() {
                String::new()
            } else {
                value.to_string()
            },
        }
    }
    fn form(&mut self, title: &str, action: &str, fields: Vec<Field>) {
        self.form = Some(Form {
            title: title.into(),
            action: action.into(),
            fields,
            selected: 0,
        });
    }
    fn open(&mut self, command: &str) -> Result<()> {
        self.selected = 0;
        self.scroll = 0;
        self.task = None;
        match command {
            "queue" | "history" => self.screen = command.into(),
            "theme" => self.form(
                "Theme",
                "theme",
                vec![Self::field("theme", "string", self.cfg["theme"].clone())],
            ),
            "clear-history" => self.confirm = Some("clear-history".into()),
            "doctor" => self.message(
                "Dependencies",
                serde_json::to_string_pretty(&crate::cli::dependencies(&self.cfg))?,
            ),
            "library" => self.message(
                "Downloads",
                serde_json::to_string_pretty(&crate::daemon::library(&self.cfg)?)?,
            ),
            "presets" => self.message(
                "Presets",
                serde_json::to_string_pretty(&config::media_presets()?)?,
            ),
            "settings" => {
                let defaults = config::defaults();
                let fields = self
                    .cfg
                    .as_object()
                    .unwrap()
                    .iter()
                    .filter(|(key, _)| !key.contains("presets") && *key != "preset_defaults")
                    .map(|(key, value)| {
                        Self::field(
                            key,
                            if defaults[key].is_boolean() {
                                "bool"
                            } else if defaults[key].is_number() {
                                "int"
                            } else {
                                "string"
                            },
                            value.clone(),
                        )
                    })
                    .collect();
                self.form("Settings", "settings", fields);
            }
            "download" | "playlist" | "audio-download" | "thumbnail" => {
                let mut fields = vec![
                    Self::field("url", "string", json!("")),
                    Self::field("output_dir", "string", self.cfg["download_dir"].clone()),
                ];
                let initial = if self.cfg["preset_defaults"].is_object() {
                    self.cfg["preset_defaults"].clone()
                } else {
                    config::download_fields()
                };
                for (key, value) in initial.as_object().unwrap() {
                    let value = if key == "audio_only" && command == "audio-download" {
                        json!(true)
                    } else {
                        value.clone()
                    };
                    fields.push(Self::field(
                        key,
                        if value.is_boolean() { "bool" } else { "string" },
                        value,
                    ));
                }
                if command == "thumbnail" {
                    fields.push(Self::field(
                        "thumbnail_format",
                        "string",
                        self.cfg["thumbnail_format"].clone(),
                    ));
                }
                self.form("Download", command, fields);
            }
            "editor" | "pipeline" => {
                self.editor = Editor {
                    name: command.into(),
                    ..Default::default()
                };
                if self.editor.path().exists() {
                    self.confirm = Some("restore-editor".into());
                }
                self.screen = "editor".into();
            }
            "ranges" => self.form(
                "Ranges",
                "open-ranges",
                vec![Self::field("input", "string", json!(""))],
            ),
            "batch" => self.form(
                "Batch",
                "batch",
                vec![
                    Self::field("directory", "string", json!("")),
                    Self::field("output_dir", "string", json!("")),
                    Self::field("project", "string", json!("")),
                    Self::field("preset", "string", json!("standard_mp4")),
                    Self::field("skip_invalid", "bool", json!(false)),
                ],
            ),
            "probe" => self.form(
                "Inspect",
                "probe",
                vec![Self::field("input", "string", json!(""))],
            ),
            "convert" => {
                let presets = config::convert_presets();
                self.form(
                    "Convert",
                    "convert",
                    vec![
                        Self::field("input", "string", json!("")),
                        Self::field("preset", "string", presets[0]["id"].clone()),
                        Self::field("output", "string", json!("")),
                    ],
                );
            }
            "size" => self.form(
                "Export size",
                "size",
                vec![
                    Self::field("input", "string", json!("")),
                    Self::field("size_mb", "number", json!(50)),
                    Self::field("two_pass", "bool", json!(true)),
                    Self::field("output", "string", json!("")),
                ],
            ),
            other => self.operation_form(other, false)?,
        }
        Ok(())
    }
    fn operation_form(&mut self, kind: &str, editor: bool) -> Result<()> {
        let mut fields = Vec::new();
        if !editor {
            fields.push(Self::field("source", "string", json!("")));
            fields.push(Self::field("output", "string", json!("")));
        }
        let schema = config::schema();
        for (key, ty) in schema[kind].as_object().context("unknown operation")? {
            let default = match (kind, key.as_str()) {
                ("trim", "start") => json!("0"),
                ("trim", "end") => json!("5"),
                ("trim", "mode") => json!("reencode"),
                ("aspect", "ratio_w") => json!(9),
                ("aspect", "ratio_h") => json!(16),
                ("aspect", "mode") => json!("crop"),
                ("aspect", "size_preset") => json!("1080p"),
                ("demux", "stream_type") => json!("audio"),
                ("subtitle", "mode") => json!("burn"),
                ("scale", "width") => json!(1280),
                ("fps", "fps") => json!("30"),
                ("transcode", "container") => json!("mp4"),
                ("transcode", "vcodec") => json!("libx264"),
                ("transcode", "acodec") => json!("aac"),
                ("transcode", "crf") => json!("20"),
                _ => Value::Null,
            };
            fields.push(Self::field(key, ty.as_str().unwrap(), default));
        }
        self.form(
            kind,
            &if editor {
                format!("add:{kind}")
            } else {
                format!("media:{kind}")
            },
            fields,
        );
        Ok(())
    }
    fn enqueue(&mut self, pipeline: Pipeline, title: &str) -> Result<()> {
        let plan = media::build(&pipeline, &self.cfg)?;
        let id = self
            .queue
            .enqueue(plan, title, &pipeline.inputs.join(","), "")?;
        self.screen = "queue".into();
        self.task = Some(id);
        self.selected = self.queue.list().len().saturating_sub(1);
        Ok(())
    }
    fn submit(&mut self, form: Form) -> Result<()> {
        let mut values = json!({});
        for field in &form.fields {
            if field.value.is_empty() {
                continue;
            }
            let value = match field.kind.as_str() {
                "string" => json!(field.value),
                "[]string" => {
                    if field.value.starts_with('[') {
                        serde_json::from_str(&field.value)?
                    } else {
                        json!(field.value.lines().map(str::to_string).collect::<Vec<_>>())
                    }
                }
                _ => serde_json::from_str(&field.value)
                    .with_context(|| format!("Invalid {}", field.key))?,
            };
            values[&field.key] = value;
        }
        let value = |key: &str| string(&values, key);
        match form.action.as_str() {
            "theme" => {
                let mut updated = self.cfg.clone();
                updated["theme"] = values["theme"].clone();
                config::save(&updated)?;
                self.cfg = updated;
            }
            "history-search" => {
                self.history_query = value("query");
                self.selected = 0;
                self.screen = "history".into();
            }
            "settings" => {
                let mut updated = self.cfg.clone();
                config::merge(&mut updated, &values)?;
                config::save(&updated)?;
                self.queue
                    .set_maximum(number(&updated, "bg_queue_max", 3.0) as usize);
                self.cfg = updated;
            }
            "download" | "playlist" | "audio-download" | "thumbnail" => {
                let url = value("url");
                let format = value("thumbnail_format");
                let plan = download::build(
                    &url,
                    &values,
                    &self.cfg,
                    &value("output_dir"),
                    form.action == "playlist",
                    if form.action == "thumbnail" {
                        Some(format.as_str())
                    } else {
                        None
                    },
                )?;
                let id = self.queue.enqueue(plan, "Download", &url, "")?;
                self.screen = "queue".into();
                self.selected = self.queue.list().len().saturating_sub(1);
                self.task = Some(id);
            }
            "convert" => {
                let input = value("input");
                let plan =
                    media::conversion_plan(&value("preset"), &input, &value("output"), &self.cfg)?;
                let id = self.queue.enqueue(plan, "Convert", &input, "")?;
                self.screen = "queue".into();
                self.task = Some(id);
                self.selected = self.queue.list().len().saturating_sub(1);
            }
            "size" => {
                let pipeline = Pipeline {
                    inputs: vec![value("input")],
                    output: media::Output {
                        path: value("output"),
                        target_megabytes: number(&values, "size_mb", 50.0),
                        two_pass: config::boolean(&values, "two_pass"),
                        ..Default::default()
                    },
                    ..Default::default()
                };
                self.enqueue(pipeline, "Size")?;
            }
            "probe" => self.message(
                "Inspect",
                serde_json::to_string_pretty(&media::probe(&value("input"), &self.cfg)?)?,
            ),
            "editor-files" => {
                self.editor.capture();
                self.editor.pipeline.inputs = vec![value("input")];
                self.editor.pipeline.output.path = value("output");
                self.editor.save()?;
            }
            "editor-size" => {
                self.editor.capture();
                self.editor.pipeline.output.target_megabytes = number(&values, "size_mb", 0.0);
                self.editor.pipeline.output.two_pass = config::boolean(&values, "two_pass");
                self.editor.save()?;
            }
            "save-project" => {
                storage::write_json(&config::user_path(&value("path")), &self.editor.pipeline)?
            }
            "load-project" => {
                let loaded = crate::cli::read_project(&value("path"))?;
                self.editor.capture();
                self.editor.pipeline = loaded;
                self.editor.save()?;
            }
            "save-preset" => {
                let preset = json!({"id":format!("mp_{}",media::timestamp()),"name":value("name"),"operations":self.editor.pipeline.operations,"created_at":crate::queue::now()});
                storage::update_json(
                    &config::directory().join("media_presets.json"),
                    json!([]),
                    |entries| {
                        entries
                            .as_array_mut()
                            .context("invalid presets")?
                            .push(preset);
                        Ok(())
                    },
                )?;
            }
            "batch" => {
                let mut inputs = Vec::new();
                for entry in std::fs::read_dir(config::user_path(&value("directory")))? {
                    let path = entry?.path();
                    if path.is_file() {
                        inputs.push(media::text_path(&path));
                    }
                }
                let pipeline = if value("project").is_empty() {
                    media::preset_pipeline(
                        &value("preset"),
                        inputs.first().context("empty directory")?,
                    )?
                } else {
                    crate::cli::read_project(&value("project"))?
                };
                let (report, plans) = crate::cli::batch_report(
                    &inputs,
                    &pipeline,
                    &value("output_dir"),
                    "",
                    &self.cfg,
                )?;
                ensure!(
                    report["invalid_count"] == 0 || config::boolean(&values, "skip_invalid"),
                    "{}",
                    serde_json::to_string_pretty(&report)?
                );
                self.message("Batch validation", serde_json::to_string_pretty(&report)?);
                self.confirm = Some(format!("batch:{}", serde_json::to_string(&plans)?));
            }
            "open-ranges" => {
                let input = value("input");
                let info = media::probe(&input, &self.cfg)?;
                let fps = info["streams"]
                    .as_array()
                    .unwrap()
                    .iter()
                    .find(|s| s["codec_type"] == "video")
                    .map(|s| number(s, "fps", 25.0))
                    .unwrap_or(25.0);
                let mut ranges = Ranges {
                    input,
                    cursor: 0.0,
                    start: None,
                    duration: number(&info["format"], "duration", 0.0),
                    fps: fps.max(1.0),
                    ranges: Vec::new(),
                    undo: Vec::new(),
                    redo: Vec::new(),
                    wave: Vec::new(),
                };
                if let Ok(saved) = storage::read_json(&ranges.path(), json!({})) {
                    ranges.ranges =
                        serde_json::from_value(saved["ranges"].clone()).unwrap_or_default();
                }
                if ranges.ranges.is_empty() {
                    let legacy = storage::read_json(
                        &config::directory().join("ranges_draft.json"),
                        json!({}),
                    )?;
                    if legacy["Path"] == ranges.input {
                        ranges.ranges = legacy["Ranges"]
                            .as_array()
                            .map(|items| {
                                items
                                    .iter()
                                    .filter_map(|item| {
                                        Some((item["start"].as_f64()?, item["end"].as_f64()?))
                                    })
                                    .filter(|(start, end)| {
                                        *start >= 0.0 && *end > *start && *end <= ranges.duration
                                    })
                                    .collect()
                            })
                            .unwrap_or_default();
                    }
                }
                ranges.wave =
                    media::waveform(&ranges.input, 0.0, 80, &self.cfg).unwrap_or_default();
                self.ranges = Some(ranges);
                self.screen = "ranges".into();
            }
            "range-jump" => {
                let position = media::parse_time(&value("time"))?;
                let ranges = self.ranges.as_mut().unwrap();
                ranges.cursor = position.min(ranges.duration);
                ranges.wave = media::waveform(
                    &ranges.input,
                    (ranges.cursor - 30.0).max(0.0),
                    80,
                    &self.cfg,
                )?;
            }
            "export-ranges" => {
                let ranges = self.ranges.as_ref().unwrap();
                let mut args = vec![ranges.input.clone()];
                for (start, end) in &ranges.ranges {
                    args.extend(["--range".into(), format!("{start}..{end}")]);
                }
                if !value("output").is_empty() {
                    args.extend(["-o".into(), value("output")]);
                }
                if config::boolean(&values, "join") {
                    args.push("--join".into());
                }
                let parsed = crate::cli::Arguments::parse(&args, "ranges")?;
                let plans = crate::cli::ranges_plans(&parsed, &self.cfg)?;
                for plan in plans {
                    self.queue
                        .enqueue(plan, "Ranges", &ranges_source(&parsed), "")?;
                }
                self.screen = "queue".into();
            }
            action if action.starts_with("add:") || action.starts_with("media:") => {
                let editor = action.starts_with("add:");
                let kind = action.split_once(':').unwrap().1;
                let mut options = values.clone();
                let source = value("source");
                let output = value("output");
                options.as_object_mut().unwrap().remove("source");
                options.as_object_mut().unwrap().remove("output");
                let operation = media::operation(kind, options);
                media::validate_operation(&operation)?;
                if editor {
                    self.editor.capture();
                    self.editor.pipeline.operations.push(operation);
                    self.editor.save()?;
                } else {
                    let inputs = if kind == "concat" {
                        serde_json::from_value(operation.config["inputs"].clone())?
                    } else {
                        vec![source]
                    };
                    let pipeline = Pipeline {
                        inputs,
                        operations: vec![operation],
                        output: media::Output {
                            path: output,
                            ..Default::default()
                        },
                        ..Default::default()
                    };
                    self.enqueue(pipeline, kind)?;
                }
            }
            _ => {}
        }
        Ok(())
    }
    fn key(&mut self, key: KeyEvent) -> Result<()> {
        if self.error.take().is_some() {
            return Ok(());
        }
        if key.kind == KeyEventKind::Release {
            return Ok(());
        }
        let control = key.modifiers.contains(KeyModifiers::CONTROL);
        if self.confirm.is_some() {
            let confirm = self.confirm.take().unwrap();
            if matches!(
                key.code,
                KeyCode::Char('y') | KeyCode::Char('д') | KeyCode::Enter
            ) {
                if confirm == "quit" {
                    self.quit = true;
                } else if confirm == "clear-history" {
                    storage::write_json(&config::directory().join("history.json"), &json!([]))?;
                } else if confirm == "restore-editor" {
                    self.editor.pipeline =
                        crate::cli::read_project(&media::text_path(&self.editor.path()))?;
                } else if let Some(plans) = confirm.strip_prefix("batch:") {
                    let plans: Vec<media::Plan> = serde_json::from_str(plans)?;
                    let group = media::timestamp();
                    for plan in plans {
                        self.queue.enqueue(plan, "Batch", "", &group)?;
                    }
                    self.screen = "queue".into();
                }
            }
            return Ok(());
        }
        if let Some(mut picker) = self.picker.take() {
            match key.code {
                KeyCode::Esc => return Ok(()),
                KeyCode::Up => picker.selected = picker.selected.saturating_sub(1),
                KeyCode::Down => {
                    picker.selected =
                        (picker.selected + 1).min(picker.entries.len().saturating_sub(1))
                }
                KeyCode::Backspace => {
                    if let Some(parent) = picker.directory.parent() {
                        picker = Picker::read(parent.into())?;
                    }
                }
                KeyCode::Enter => {
                    let selected = if key.modifiers.contains(KeyModifiers::ALT) {
                        Some(picker.directory.clone())
                    } else {
                        picker.entries.get(picker.selected).cloned()
                    };
                    if let Some(path) = selected {
                        if path.is_dir() && !key.modifiers.contains(KeyModifiers::ALT) {
                            self.picker = Some(Picker::read(path)?);
                            return Ok(());
                        }
                        if let Some(form) = self.form.as_mut() {
                            form.fields[form.selected].value = media::text_path(&path);
                        }
                        return Ok(());
                    }
                }
                _ => {}
            }
            self.picker = Some(picker);
            return Ok(());
        }
        if let Some(mut form) = self.form.take() {
            let last = form.fields.len().saturating_sub(1);
            match key.code {
                KeyCode::Esc => return Ok(()),
                KeyCode::F(2) => {
                    let current = config::user_path(&form.fields[form.selected].value);
                    let directory = if current.is_dir() {
                        current
                    } else {
                        current
                            .parent()
                            .filter(|p| p.is_dir())
                            .map(PathBuf::from)
                            .unwrap_or(std::env::current_dir()?)
                    };
                    self.picker = Some(Picker::read(directory)?);
                }
                KeyCode::Left | KeyCode::Right => {
                    let field = &mut form.fields[form.selected];
                    let choices = field_choices(&form.action, &field.key, &self.cfg);
                    if !choices.is_empty() {
                        let index = choices
                            .iter()
                            .position(|value| value == &field.value)
                            .unwrap_or(0);
                        let next = if key.code == KeyCode::Right {
                            (index + 1) % choices.len()
                        } else {
                            (index + choices.len() - 1) % choices.len()
                        };
                        field.value = choices[next].clone();
                    }
                }
                KeyCode::Char('s') if control => {
                    let backup = Form {
                        title: form.title.clone(),
                        action: form.action.clone(),
                        fields: form.fields.clone(),
                        selected: form.selected,
                    };
                    if let Err(error) = self.submit(form) {
                        self.form = Some(backup);
                        return Err(error);
                    }
                    return Ok(());
                }
                KeyCode::Enter if control => {
                    self.submit(form)?;
                    return Ok(());
                }
                KeyCode::Enter | KeyCode::Tab | KeyCode::Down => {
                    form.selected = (form.selected + 1).min(last)
                }
                KeyCode::BackTab | KeyCode::Up => form.selected = form.selected.saturating_sub(1),
                KeyCode::Char('u') if control => form.fields[form.selected].value.clear(),
                KeyCode::Backspace => {
                    form.fields[form.selected].value.pop();
                }
                KeyCode::Char(' ') if form.fields[form.selected].kind == "bool" => {
                    let field = &mut form.fields[form.selected];
                    field.value = if field.value == "true" {
                        "false".into()
                    } else {
                        "true".into()
                    };
                }
                KeyCode::Char(character) if !control => {
                    form.fields[form.selected].value.push(character)
                }
                _ => {}
            }
            self.form = Some(form);
            return Ok(());
        }
        if let Some(mut search) = self.palette.take() {
            match key.code {
                KeyCode::Esc | KeyCode::F(12) => return Ok(()),
                KeyCode::Char('p') if control => return Ok(()),
                KeyCode::Char('u') if control => {
                    search.clear();
                    self.palette_selected = 0;
                }
                KeyCode::Backspace => {
                    search.pop();
                    self.palette_selected = 0;
                }
                KeyCode::Up => self.palette_selected = self.palette_selected.saturating_sub(1),
                KeyCode::Down => self.palette_selected += 1,
                KeyCode::Enter => {
                    let filtered = self.filtered(&search);
                    if let Some(action) =
                        filtered.get(self.palette_selected.min(filtered.len().saturating_sub(1)))
                    {
                        let command = action.command;
                        self.open(command)?;
                    }
                    return Ok(());
                }
                KeyCode::Char(c) if !control => {
                    search.push(c);
                    self.palette_selected = 0;
                }
                _ => {}
            }
            self.palette = Some(search);
            return Ok(());
        }
        if key.code == KeyCode::F(12) || (control && key.code == KeyCode::Char('p')) {
            self.palette = Some(String::new());
            self.palette_selected = 0;
            return Ok(());
        }
        if key.modifiers.contains(KeyModifiers::ALT) && key.code == KeyCode::Char('3') {
            return self.open("editor");
        }
        if control && key.code == KeyCode::Char('c') {
            if self.screen == "queue" {
                if let Some(task) = self.queue.list().get(self.selected) {
                    self.queue.cancel(task.id);
                }
            } else {
                self.quit_or_confirm();
            }
            return Ok(());
        }
        if control
            && (key.code == KeyCode::Char('z') || key.code == KeyCode::Char('y'))
            && self.screen == "editor"
        {
            return self.editor.move_history(key.code == KeyCode::Char('y'));
        }
        match self.screen.as_str() {
            "home" => match key.code {
                KeyCode::Left | KeyCode::Char('h') => {
                    self.tab = (self.tab + 2) % 3;
                    self.selected = 0;
                }
                KeyCode::Right | KeyCode::Char('l') | KeyCode::Tab => {
                    self.tab = (self.tab + 1) % 3;
                    self.selected = 0;
                }
                KeyCode::Up | KeyCode::Char('k') => self.selected = self.selected.saturating_sub(1),
                KeyCode::Down | KeyCode::Char('j') => self.selected += 1,
                KeyCode::Enter => {
                    let items: Vec<_> = actions()
                        .into_iter()
                        .filter(|a| a.category == self.tab)
                        .collect();
                    if let Some(action) =
                        items.get(self.selected.min(items.len().saturating_sub(1)))
                    {
                        self.open(action.command)?;
                    }
                }
                KeyCode::Esc | KeyCode::Char('q') => self.quit_or_confirm(),
                _ => {}
            },
            "queue" => match key.code {
                KeyCode::Esc | KeyCode::Char('q') => {
                    self.screen = "home".into();
                    self.task = None;
                }
                KeyCode::Up => {
                    self.selected = self.selected.saturating_sub(1);
                    self.scroll = 0;
                }
                KeyCode::Down => {
                    self.selected =
                        (self.selected + 1).min(self.queue.list().len().saturating_sub(1));
                    self.scroll = 0;
                }
                KeyCode::Char('c') | KeyCode::Delete => {
                    if let Some(task) = self.queue.list().get(self.selected) {
                        self.queue.cancel(task.id);
                    }
                }
                KeyCode::Char('r') => {
                    if let Some(task) = self.queue.list().get(self.selected) {
                        self.queue.retry(task.id)?;
                    }
                }
                KeyCode::F(10) | KeyCode::Enter => {
                    if self.task.is_some() {
                        self.task = None;
                    } else {
                        self.task = self.queue.list().get(self.selected).map(|t| t.id);
                    }
                }
                KeyCode::PageUp => self.scroll = self.scroll.saturating_sub(10),
                KeyCode::PageDown => self.scroll = self.scroll.saturating_add(10),
                _ => {}
            },
            "history" => match key.code {
                KeyCode::Char('/') => self.form(
                    "Search history",
                    "history-search",
                    vec![Self::field("query", "string", json!(self.history_query))],
                ),
                KeyCode::Esc | KeyCode::Char('q') => self.screen = "home".into(),
                KeyCode::Up => self.selected = self.selected.saturating_sub(1),
                KeyCode::Down => self.selected += 1,
                KeyCode::Char('c') => self.confirm = Some("clear-history".into()),
                KeyCode::Delete => {
                    let history = self.history_entries()?;
                    if let Some(entry) = history.get(self.selected) {
                        crate::daemon::delete_history(entry, false, &self.cfg)?;
                    }
                }
                KeyCode::Enter => {
                    let history = self.history_entries()?;
                    if let Some(entry) = history.get(self.selected) {
                        self.message("History", serde_json::to_string_pretty(entry)?);
                    }
                }
                KeyCode::Char('r') => {
                    let history = self.history_entries()?;
                    if let Some(entry) = history.get(self.selected) {
                        let commands = serde_json::from_value(entry["commands"].clone())
                            .context("no saved command")?;
                        self.queue.replay(
                            media::Plan {
                                commands,
                                target: string(entry, "target"),
                                replacement: None,
                                external: None,
                            },
                            &string(entry, "type"),
                            &string(entry, "source"),
                        )?;
                        self.screen = "queue".into();
                    }
                }
                _ => {}
            },
            "editor" => match key.code {
                KeyCode::Esc | KeyCode::Char('q') => self.screen = "home".into(),
                KeyCode::Up => self.selected = self.selected.saturating_sub(1),
                KeyCode::Down => self.selected += 1,
                KeyCode::Char('i') => self.form(
                    "Input / output",
                    "editor-files",
                    vec![
                        Self::field(
                            "input",
                            "string",
                            json!(
                                self.editor
                                    .pipeline
                                    .inputs
                                    .first()
                                    .cloned()
                                    .unwrap_or_default()
                            ),
                        ),
                        Self::field("output", "string", json!(self.editor.pipeline.output.path)),
                    ],
                ),
                KeyCode::Char('a') => {
                    self.screen = "add-operation".into();
                    self.selected = 0;
                }
                KeyCode::Delete => {
                    if self.selected < self.editor.pipeline.operations.len() {
                        self.editor.capture();
                        self.editor.pipeline.operations.remove(self.selected);
                        self.editor.save()?;
                    }
                }
                KeyCode::Char('s') => self.form(
                    "Save project",
                    "save-project",
                    vec![Self::field("path", "string", json!("project.json"))],
                ),
                KeyCode::Char('l') => self.form(
                    "Load project",
                    "load-project",
                    vec![Self::field("path", "string", json!("project.json"))],
                ),
                KeyCode::Char('p') => self.form(
                    "Save preset",
                    "save-preset",
                    vec![Self::field("name", "string", json!(""))],
                ),
                KeyCode::Char('m') => self.form(
                    "Export size",
                    "editor-size",
                    vec![
                        Self::field(
                            "size_mb",
                            "number",
                            json!(self.editor.pipeline.output.target_megabytes),
                        ),
                        Self::field(
                            "two_pass",
                            "bool",
                            json!(self.editor.pipeline.output.two_pass),
                        ),
                    ],
                ),
                KeyCode::Char('d') => {
                    let plan = media::build(&self.editor.pipeline, &self.cfg)?;
                    self.message(
                        "Commands",
                        plan.commands
                            .iter()
                            .map(|c| media::display_command(c))
                            .collect::<Vec<_>>()
                            .join("\n\n"),
                    );
                }
                KeyCode::Enter => self.enqueue(self.editor.pipeline.clone(), "Pipeline")?,
                _ => {}
            },
            "add-operation" => {
                let kinds: Vec<_> = config::schema()
                    .as_object()
                    .unwrap()
                    .keys()
                    .cloned()
                    .collect();
                match key.code {
                    KeyCode::Esc => self.screen = "editor".into(),
                    KeyCode::Up => self.selected = self.selected.saturating_sub(1),
                    KeyCode::Down => self.selected = (self.selected + 1).min(kinds.len() - 1),
                    KeyCode::Enter => {
                        let kind = &kinds[self.selected.min(kinds.len() - 1)];
                        self.operation_form(kind, true)?;
                        self.screen = "editor".into();
                    }
                    _ => {}
                }
            }
            "ranges" => {
                let ranges = self.ranges.as_mut().unwrap();
                match key.code {
                    KeyCode::Esc | KeyCode::Char('q') => self.screen = "home".into(),
                    KeyCode::Left => ranges.cursor = (ranges.cursor - 1.0 / ranges.fps).max(0.0),
                    KeyCode::Right => {
                        ranges.cursor = (ranges.cursor + 1.0 / ranges.fps).min(ranges.duration)
                    }
                    KeyCode::Up => ranges.cursor = (ranges.cursor + 1.0).min(ranges.duration),
                    KeyCode::Down => ranges.cursor = (ranges.cursor - 1.0).max(0.0),
                    KeyCode::Char('i' | 'I') => ranges.start = Some(ranges.cursor),
                    KeyCode::Char('o' | 'O') => {
                        let start = ranges.start.context("mark start with I")?;
                        ensure!(ranges.cursor > start, "end must be after start");
                        ranges.capture();
                        ranges.ranges.push((start, ranges.cursor));
                        ranges.start = None;
                        ranges.save()?;
                    }
                    KeyCode::Char('d' | 'D') => {
                        ranges.capture();
                        ranges.ranges.pop();
                        ranges.save()?;
                    }
                    KeyCode::Char('z') if control => {
                        if let Some(previous) = ranges.undo.pop() {
                            ranges.redo.push(ranges.ranges.clone());
                            ranges.ranges = previous;
                            ranges.save()?;
                        }
                    }
                    KeyCode::Char('y') if control => {
                        if let Some(next) = ranges.redo.pop() {
                            ranges.undo.push(ranges.ranges.clone());
                            ranges.ranges = next;
                            ranges.save()?;
                        }
                    }
                    KeyCode::Char('g' | 'G') => self.form(
                        "Jump to time",
                        "range-jump",
                        vec![Self::field("time", "string", json!("0"))],
                    ),
                    KeyCode::Enter => {
                        ensure!(!ranges.ranges.is_empty(), "mark at least one range");
                        self.form(
                            "Export ranges",
                            "export-ranges",
                            vec![
                                Self::field("join", "bool", json!(false)),
                                Self::field("output", "string", json!("")),
                            ],
                        );
                    }
                    _ => {}
                }
            }
            _ => match key.code {
                KeyCode::Esc | KeyCode::Char('q') => self.screen = "home".into(),
                KeyCode::Up => self.scroll = self.scroll.saturating_sub(1),
                KeyCode::Down => self.scroll = self.scroll.saturating_add(1),
                KeyCode::PageUp => self.scroll = self.scroll.saturating_sub(10),
                KeyCode::PageDown => self.scroll = self.scroll.saturating_add(10),
                _ => {}
            },
        }
        Ok(())
    }
    fn history_entries(&self) -> Result<Vec<Value>> {
        let query = self.history_query.to_lowercase();
        Ok(config::history()?
            .as_array()
            .context("invalid history")?
            .iter()
            .filter(|entry| entry.to_string().to_lowercase().contains(&query))
            .cloned()
            .collect())
    }
    fn filtered(&self, search: &str) -> Vec<Action> {
        let query = search.to_lowercase();
        let terms: Vec<_> = query.split_whitespace().collect();
        actions()
            .into_iter()
            .filter(|action| {
                let searchable =
                    format!("{} {} {}", action.command, action.ru, action.en).to_lowercase();
                terms.iter().all(|term| searchable.contains(term))
            })
            .collect()
    }
    fn quit_or_confirm(&mut self) {
        if self.queue.list().iter().any(|t| t.active()) {
            self.confirm = Some("quit".into());
        } else {
            self.quit = true;
        }
    }
    fn ascii_logo(&self) -> String {
        if string(&self.cfg, "logo_mode") == "none" {
            return String::new();
        }
        let logos: Value = serde_json::from_str(include_str!("../resources/logos.json")).unwrap();
        let preset = string(&self.cfg, "logo_ascii_preset");
        logos[&preset]
            .as_array()
            .map(|lines| {
                lines
                    .iter()
                    .filter_map(Value::as_str)
                    .collect::<Vec<_>>()
                    .join("\n")
            })
            .unwrap_or("MediaCLI".into())
    }
    fn accent(&self) -> Color {
        match string(&self.cfg, "theme").as_str() {
            "nord" => Color::Rgb(136, 192, 208),
            "dracula" => Color::Rgb(189, 147, 249),
            "gruvbox" => Color::Rgb(250, 189, 47),
            "classic" => Color::White,
            "matrix" => Color::Green,
            "amber" => Color::Yellow,
            "fire" => Color::Red,
            "purple" => Color::Magenta,
            _ => Color::Cyan,
        }
    }
    fn draw(&mut self, frame: &mut ratatui::Frame) {
        let area = frame.area();
        if area.width < 24 || area.height < 8 {
            frame.render_widget(Paragraph::new("MediaCLI: enlarge terminal"), area);
            return;
        }
        let accent = self.accent();
        if !config::boolean(&self.cfg, "use_terminal_bg") {
            frame.render_widget(
                Block::default().style(Style::default().bg(Color::Rgb(16, 18, 24))),
                area,
            );
        }
        let style = Style::default().fg(accent);
        let highlight = style.add_modifier(Modifier::BOLD | Modifier::REVERSED);
        let layout = Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(3),
                Constraint::Min(1),
                Constraint::Length(2),
            ])
            .split(area);
        let tabs = if self.ru() {
            vec!["Загрузки", "Медиа", "Система"]
        } else {
            vec!["Downloads", "Media", "System"]
        };
        frame.render_widget(
            Tabs::new(tabs)
                .select(self.tab)
                .highlight_style(highlight)
                .block(
                    Block::default()
                        .borders(Borders::BOTTOM)
                        .title(format!(" MediaCLI {} ", crate::VERSION)),
                ),
            layout[0],
        );
        let body = layout[1];
        let mut footer = self
            .tr(
                "↑↓ Выбрать  Enter Открыть  Ctrl+P Палитра  Esc Назад",
                "↑↓ Select  Enter Open  Ctrl+P Palette  Esc Back",
            )
            .to_string();
        match self.screen.as_str() {
            "home" => {
                let columns = Layout::default()
                    .direction(Direction::Horizontal)
                    .constraints([Constraint::Percentage(45), Constraint::Percentage(55)])
                    .split(body);
                let items: Vec<_> = actions()
                    .into_iter()
                    .filter(|a| a.category == self.tab)
                    .map(|a| ListItem::new(if self.ru() { a.ru } else { a.en }))
                    .collect();
                self.selected = self.selected.min(items.len().saturating_sub(1));
                frame.render_stateful_widget(
                    List::new(items)
                        .block(
                            Block::default()
                                .borders(Borders::ALL)
                                .title(self.tr(" Инструменты ", " Tools ")),
                        )
                        .highlight_style(highlight)
                        .highlight_symbol("› "),
                    columns[0],
                    &mut ListState::default().with_selected(Some(self.selected)),
                );
                let tasks = self.queue.list();
                let running = tasks.iter().filter(|t| t.active()).count();
                let description = format!(
                    "{}\n\n{}: {running}\n\n{}\n\nyt-dlp · FFmpeg\n\n{}",
                    self.ascii_logo(),
                    self.tr("Активных задач", "Active tasks"),
                    self.tr(
                        "Выберите инструмент слева.\nAlt+3 — универсальный редактор.",
                        "Choose a tool on the left.\nAlt+3 — universal editor."
                    ),
                    self.tr(
                        "Ctrl+P / F12 — поиск действий",
                        "Ctrl+P / F12 — search actions"
                    )
                );
                frame.render_widget(
                    Paragraph::new(description)
                        .block(
                            Block::default()
                                .borders(Borders::ALL)
                                .title(self.tr(" Состояние ", " Status ")),
                        )
                        .wrap(Wrap { trim: false }),
                    columns[1],
                );
            }
            "queue" => {
                let tasks = self.queue.list();
                self.selected = self.selected.min(tasks.len().saturating_sub(1));
                let regions = Layout::default()
                    .constraints([Constraint::Percentage(45), Constraint::Percentage(55)])
                    .split(body);
                let items: Vec<_> = tasks
                    .iter()
                    .map(|t| {
                        ListItem::new(format!(
                            "#{} {:10} {:5.1}% {}",
                            t.id, t.status, t.progress, t.title
                        ))
                    })
                    .collect();
                frame.render_stateful_widget(
                    List::new(items)
                        .block(
                            Block::default()
                                .borders(Borders::ALL)
                                .title(self.tr(" Очередь ", " Queue ")),
                        )
                        .highlight_style(highlight),
                    regions[0],
                    &mut ListState::default().with_selected(Some(self.selected)),
                );
                if let Some(task) = tasks.get(self.selected) {
                    let text = if self.task.is_some() {
                        task.log_tail.join("\n")
                    } else {
                        format!(
                            "{}\n{}\n\n{} → {}\n\n{:.1}%\n{}",
                            task.title,
                            task.stage,
                            task.source,
                            task.target,
                            task.progress,
                            task.log_tail
                                .iter()
                                .rev()
                                .take(5)
                                .cloned()
                                .collect::<Vec<_>>()
                                .into_iter()
                                .rev()
                                .collect::<Vec<_>>()
                                .join("\n")
                        )
                    };
                    frame.render_widget(
                        Paragraph::new(text)
                            .scroll((self.scroll, 0))
                            .wrap(Wrap { trim: false })
                            .block(
                                Block::default()
                                    .borders(Borders::ALL)
                                    .title(" Task / logs "),
                            ),
                        regions[1],
                    );
                }
                footer = self
                    .tr(
                        "C / Ctrl+C Отменить  R Повторить  F10 Журнал  Esc В фон",
                        "C / Ctrl+C Cancel  R Retry  F10 Logs  Esc Background",
                    )
                    .into();
            }
            "history" => {
                let entries = self.history_entries().unwrap_or_default();
                self.selected = self.selected.min(entries.len().saturating_sub(1));
                let items: Vec<_> = entries
                    .iter()
                    .map(|e| {
                        ListItem::new(format!(
                            "{} {} {} → {}",
                            string(e, "time"),
                            string(e, "status"),
                            string(e, "source"),
                            string(e, "target")
                        ))
                    })
                    .collect();
                frame.render_stateful_widget(
                    List::new(items)
                        .block(
                            Block::default()
                                .borders(Borders::ALL)
                                .title(self.tr(" История ", " History ")),
                        )
                        .highlight_style(highlight),
                    body,
                    &mut ListState::default().with_selected(Some(self.selected)),
                );
                footer = self
                    .tr(
                        "/ Поиск  R Повторить  Delete Удалить запись  C Очистить  Esc Назад",
                        "/ Search  R Retry  Delete Remove entry  C Clear  Esc Back",
                    )
                    .into();
            }
            "editor" => {
                let input = self.editor.pipeline.inputs.join(", ");
                let mut lines = vec![
                    Line::from(format!("Input: {input}")),
                    Line::from(format!("Output: {}", self.editor.pipeline.output.path)),
                    Line::from(format!(
                        "Target MB: {}",
                        self.editor.pipeline.output.target_megabytes
                    )),
                    Line::from(""),
                ];
                for (index, op) in self.editor.pipeline.operations.iter().enumerate() {
                    lines.push(Line::styled(
                        format!("{}. {} {}", index + 1, op.kind, op.config),
                        if index == self.selected {
                            highlight
                        } else {
                            Style::default()
                        },
                    ));
                }
                frame.render_widget(
                    Paragraph::new(lines).wrap(Wrap { trim: false }).block(
                        Block::default()
                            .borders(Borders::ALL)
                            .title(format!(" {} ", self.editor.name)),
                    ),
                    body,
                );
                footer=self.tr("I Файлы  A Операция  Del Удалить  S Сохранить  L Открыть\nM Размер  P Набор  D Команды  Ctrl+Z/Y Отмена/повтор  Enter Экспорт","I Files  A Add  Del Remove  S Save  L Load\nM Size  P Preset  D Commands  Ctrl+Z/Y Undo/redo  Enter Export").into();
            }
            "add-operation" => {
                let schema = config::schema();
                let items: Vec<_> = schema
                    .as_object()
                    .unwrap()
                    .keys()
                    .map(|k| ListItem::new(k.clone()))
                    .collect();
                frame.render_stateful_widget(
                    List::new(items)
                        .block(Block::default().borders(Borders::ALL).title(" Operation "))
                        .highlight_style(highlight),
                    body,
                    &mut ListState::default().with_selected(Some(self.selected)),
                );
            }
            "ranges" => {
                if let Some(ranges) = &self.ranges {
                    let levels: Vec<char> = " ▁▂▃▄▅▆▇█".chars().collect();
                    let waveform: String = ranges
                        .wave
                        .iter()
                        .map(|v| levels[(v * 8.0).round().clamp(0.0, 8.0) as usize])
                        .collect();
                    let mut text = format!(
                        "{}\n\n{:.3} / {:.3}s    {:.3} fps\nStart: {:?}\n\n{waveform}\n\n",
                        ranges.input, ranges.cursor, ranges.duration, ranges.fps, ranges.start
                    );
                    for (index, (start, end)) in ranges.ranges.iter().enumerate() {
                        text.push_str(&format!("{}. {start:.3} .. {end:.3}\n", index + 1));
                    }
                    frame.render_widget(
                        Paragraph::new(text).wrap(Wrap { trim: false }).block(
                            Block::default()
                                .borders(Borders::ALL)
                                .title(" Ranges / waveform "),
                        ),
                        body,
                    );
                }
                footer=self.tr("←→ Кадр  ↑↓ Секунда  I Начало  O Конец  D Удалить\nG Перейти  Ctrl+Z/Y Отмена/повтор  Enter Экспорт","←→ Frame  ↑↓ Second  I Start  O End  D Remove\nG Jump  Ctrl+Z/Y Undo/redo  Enter Export").into();
            }
            _ => frame.render_widget(
                Paragraph::new(self.text.clone())
                    .scroll((self.scroll, 0))
                    .wrap(Wrap { trim: false })
                    .block(
                        Block::default()
                            .borders(Borders::ALL)
                            .title(format!(" {} ", self.title)),
                    ),
                body,
            ),
        }
        frame.render_widget(Paragraph::new(footer).style(style), layout[2]);
        if let Some(form) = &self.form {
            frame.render_widget(Clear, body);
            let visible = body.height.saturating_sub(4) as usize;
            let first = form.selected.saturating_sub(visible.saturating_sub(1));
            let lines: Vec<_> = form
                .fields
                .iter()
                .enumerate()
                .skip(first)
                .take(visible)
                .map(|(index, field)| {
                    let value = if field.key.contains("password") || field.key == "proxy_url" {
                        "•".repeat(field.value.chars().count())
                    } else {
                        field.value.clone()
                    };
                    Line::from(vec![
                        Span::styled(format!("{}: ", field.key), style),
                        Span::styled(
                            value,
                            if index == form.selected {
                                highlight
                            } else {
                                Style::default()
                            },
                        ),
                    ])
                })
                .collect();
            frame.render_widget(
                Paragraph::new(lines).block(
                    Block::default()
                        .borders(Borders::ALL)
                        .title(format!(" {} ", form.title)),
                ),
                body,
            );
            frame.render_widget(
                Paragraph::new(self.tr(
                    "↑↓ Поле  ←→ Вариант  F2 Файлы  Ctrl+U Очистить  Ctrl+S Применить  Esc Отмена",
                    "↑↓ Field  ←→ Choice  F2 Files  Ctrl+U Clear  Ctrl+S Apply  Esc Cancel",
                ))
                .style(style),
                layout[2],
            );
        }
        if let Some(search) = &self.palette {
            let popup = centered(area, 80, 80);
            frame.render_widget(Clear, popup);
            let regions = Layout::default()
                .constraints([Constraint::Length(3), Constraint::Min(1)])
                .split(popup);
            frame.render_widget(
                Paragraph::new(search.as_str()).block(
                    Block::default()
                        .borders(Borders::ALL)
                        .title(self.tr(" Поиск действий ", " Search actions ")),
                ),
                regions[0],
            );
            let filtered = self.filtered(search);
            self.palette_selected = self.palette_selected.min(filtered.len().saturating_sub(1));
            let items: Vec<_> = filtered
                .iter()
                .map(|a| ListItem::new(if self.ru() { a.ru } else { a.en }))
                .collect();
            frame.render_stateful_widget(
                List::new(items)
                    .block(Block::default().borders(Borders::ALL))
                    .highlight_style(highlight),
                regions[1],
                &mut ListState::default().with_selected(Some(self.palette_selected)),
            );
        }
        if let Some(picker) = &self.picker {
            frame.render_widget(Clear, body);
            let items: Vec<_> = picker
                .entries
                .iter()
                .map(|path| {
                    ListItem::new(format!(
                        "{} {}",
                        if path.is_dir() { "▸" } else { " " },
                        path.file_name().unwrap_or_default().to_string_lossy()
                    ))
                })
                .collect();
            frame.render_stateful_widget(
                List::new(items)
                    .block(
                        Block::default()
                            .borders(Borders::ALL)
                            .title(media::text_path(&picker.directory)),
                    )
                    .highlight_style(highlight),
                body,
                &mut ListState::default().with_selected(Some(picker.selected)),
            );
            frame.render_widget(
                Paragraph::new(self.tr(
                    "Enter Открыть/выбрать  Alt+Enter Выбрать папку  Backspace Вверх  Esc Назад",
                    "Enter Open/select  Alt+Enter Select folder  Backspace Parent  Esc Back",
                ))
                .style(style),
                layout[2],
            );
        }
        if let Some(error) = &self.error {
            let popup = centered(area, 85, 65);
            frame.render_widget(Clear, popup);
            frame.render_widget(
                Paragraph::new(format!("{error}\n\nEnter / Esc"))
                    .wrap(Wrap { trim: false })
                    .block(
                        Block::default()
                            .borders(Borders::ALL)
                            .title(self.tr(" Ошибка ", " Error ")),
                    ),
                popup,
            );
        }
        if let Some(confirm) = &self.confirm {
            let text = if confirm == "restore-editor" {
                self.tr("Восстановить сохранённый проект?", "Restore saved project?")
            } else if confirm.starts_with("batch:") {
                self.tr("Запустить проверенные файлы?", "Start validated files?")
            } else if confirm == "clear-history" {
                self.tr(
                    "Очистить историю? Файлы сохранятся.",
                    "Clear history? Media files are retained.",
                )
            } else {
                self.tr(
                    "Прервать активные задачи и выйти?",
                    "Interrupt active tasks and quit?",
                )
            };
            let popup = centered(area, 75, 35);
            frame.render_widget(Clear, popup);
            frame.render_widget(
                Paragraph::new(format!("{text}\n\nEnter / Y — Yes    Esc / N — No"))
                    .wrap(Wrap { trim: false })
                    .block(Block::default().borders(Borders::ALL).title(" Confirm ")),
                popup,
            );
        }
    }
}
fn centered(area: Rect, width: u16, height: u16) -> Rect {
    let columns = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([
            Constraint::Percentage((100 - width) / 2),
            Constraint::Percentage(width),
            Constraint::Percentage((100 - width) / 2),
        ])
        .split(area);
    Layout::default()
        .constraints([
            Constraint::Percentage((100 - height) / 2),
            Constraint::Percentage(height),
            Constraint::Percentage((100 - height) / 2),
        ])
        .split(columns[1])[1]
}
fn ranges_source(args: &crate::cli::Arguments) -> String {
    args.positionals.first().cloned().unwrap_or_default()
}
pub async fn run() -> Result<()> {
    ensure!(
        io::stdin().is_terminal() && io::stdout().is_terminal(),
        "no interactive terminal detected (stdin/stdout is not a TTY); use --help for CLI commands"
    );
    let cfg = config::load()?;
    let queue = Queue::new(
        number(&cfg, "bg_queue_max", 3.0) as usize,
        Some(config::directory().join("tui_queue.json")),
    )?;
    terminal::enable_raw_mode()?;
    let _guard = TerminalGuard;
    execute!(io::stdout(), EnterAlternateScreen, crossterm::cursor::Hide)?;
    let mut terminal = Terminal::new(CrosstermBackend::new(io::stdout()))?;
    let mut app = App::new(cfg, queue.clone());
    let result = (|| -> Result<()> {
        while !app.quit {
            terminal.draw(|frame| app.draw(frame))?;
            if event::poll(Duration::from_millis(100))?
                && let Event::Key(key) = event::read()?
                && let Err(error) = app.key(key)
            {
                app.error = Some(format!("{error:#}"));
            }
        }
        Ok(())
    })();
    queue.shutdown().await;
    result
}

fn field_choices(action: &str, key: &str, cfg: &Value) -> Vec<String> {
    let metadata = config::metadata();
    let group = match key {
        "theme" => "themes",
        "audio_format" => "audio_formats",
        "thumbnail_format" => "thumbnail_format",
        "cookies_browser" => "browsers",
        "progress_style" => "progress_styles",
        "user_goal" => "user_goals",
        "logo_protocol" => "logo_protocols",
        "logo_ascii_preset" => "logo_ascii",
        _ => "",
    };
    if let Some(values) = metadata[group].as_array() {
        return values
            .iter()
            .filter_map(|v| v.as_str().or_else(|| v["id"].as_str()))
            .map(str::to_string)
            .collect();
    }
    if key == "preset" && action == "convert" {
        return config::convert_presets()
            .as_array()
            .unwrap()
            .iter()
            .map(|p| string(p, "id"))
            .collect();
    }
    if key == "video_preset" {
        return config::video_presets()
            .as_array()
            .unwrap()
            .iter()
            .map(|p| string(p, "id"))
            .collect();
    }
    let values: Vec<&str> = match key {
        "language" => vec!["ru", "en", "en-US"],
        "cookies_mode" => vec!["none", "file", "browser"],
        "proxy_mode" => vec!["system", "none", "custom"],
        "transcode_mode" => vec!["embedded", "external"],
        "logo_mode" => vec!["ascii", "image", "none"],
        "vcodec" => vec![
            "copy",
            "libx264",
            "libx265",
            "libsvtav1",
            "prores_ks",
            "dnxhd",
        ],
        "acodec" => vec!["copy", "aac", "libopus", "flac", "pcm_s16le"],
        "container" => vec!["mp4", "mkv", "mov", "webm", "mp3", "flac", "wav"],
        "mode" if action.ends_with("aspect") => {
            vec!["crop", "letterbox", "blur", "stretch", "rotate", "flip"]
        }
        "mode" if action.ends_with("trim") => vec!["reencode", "copy"],
        "mode" if action.ends_with("subtitle") => vec!["burn", "embed", "extract"],
        "mode" if action.ends_with("concat") => vec!["filter", "demuxer"],
        "stream_type" => vec!["audio", "video", "subtitle"],
        _ => vec![],
    };
    let _ = cfg;
    values.into_iter().map(str::to_string).collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    fn app() -> App {
        App::new(config::defaults(), Queue::new(3, None).unwrap())
    }
    #[test]
    fn every_screen_and_modal_fits_small_terminals() {
        for (width, height) in [(1, 1), (23, 7), (24, 8), (40, 12), (80, 24), (160, 50)] {
            for screen in ["home", "queue", "editor", "add-operation", "text"] {
                let mut app = app();
                app.screen = screen.into();
                app.selected = usize::MAX;
                app.palette_selected = usize::MAX;
                let mut terminal =
                    Terminal::new(ratatui::backend::TestBackend::new(width, height)).unwrap();
                terminal.draw(|frame| app.draw(frame)).unwrap();
                app.palette = Some("не найдено".into());
                terminal.draw(|frame| app.draw(frame)).unwrap();
                app.palette = None;
                app.open("settings").unwrap();
                terminal.draw(|frame| app.draw(frame)).unwrap();
                app.error = Some("Invalid value".into());
                terminal.draw(|frame| app.draw(frame)).unwrap();
                app.confirm = Some("quit".into());
                terminal.draw(|frame| app.draw(frame)).unwrap();
            }
        }
    }
    #[test]
    fn palette_searches_russian_english_and_multiple_terms() {
        let app = app();
        assert!(
            app.filtered("video")
                .iter()
                .any(|a| a.command == "download")
        );
        assert!(
            app.filtered("скачать видео")
                .iter()
                .any(|a| a.command == "download")
        );
        assert!(app.filtered("QUEUE").iter().any(|a| a.command == "queue"));
        assert!(app.filtered("nonexistent").is_empty());
    }
    #[test]
    fn keyboard_forms_support_choices_and_preserve_validation_errors() {
        let mut app = app();
        app.open("theme").unwrap();
        let before = app.form.as_ref().unwrap().fields[0].value.clone();
        app.key(KeyEvent::new(KeyCode::Right, KeyModifiers::NONE))
            .unwrap();
        assert_ne!(app.form.as_ref().unwrap().fields[0].value, before);
        app.error = Some("invalid input".into());
        app.key(KeyEvent::new(KeyCode::Esc, KeyModifiers::NONE))
            .unwrap();
        assert!(app.error.is_none());
        assert!(app.form.is_some());
        app.key(KeyEvent::new(KeyCode::Esc, KeyModifiers::NONE))
            .unwrap();
        assert!(app.form.is_none());
    }
}
