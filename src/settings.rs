//! Settings categories, order and wording follow origin/main's ScreenSettingsVertical.
use crate::config::{self, boolean, string};
use ratatui::{
    Frame,
    layout::{Constraint, Direction, Layout, Rect},
    style::{Modifier, Style},
    widgets::{Block, Borders, Cell, List, ListItem, ListState, Paragraph, Row, Table, TableState},
};
use serde_json::Value;
use std::sync::OnceLock;

#[derive(Default)]
pub struct Navigation {
    pub category: usize,
    pub selected: usize,
    pub in_values: bool,
}

pub struct Setting {
    pub key: &'static str,
    pub label: String,
    pub value: String,
    pub tag: &'static str,
}

pub fn text(config: &Value, key: &str) -> String {
    static RUSSIAN: OnceLock<Value> = OnceLock::new();
    static BRITISH: OnceLock<Value> = OnceLock::new();
    static AMERICAN: OnceLock<Value> = OnceLock::new();
    let language = string(config, "language");
    let locale = match language.as_str() {
        "ru" | "ru_ru" => RUSSIAN
            .get_or_init(|| serde_json::from_str(include_str!("../locales/ru_ru.json")).unwrap()),
        "en-US" | "en_us" => AMERICAN
            .get_or_init(|| serde_json::from_str(include_str!("../locales/en_us.json")).unwrap()),
        _ => BRITISH
            .get_or_init(|| serde_json::from_str(include_str!("../locales/en_gb.json")).unwrap()),
    };
    locale[key].as_str().unwrap_or(key).to_string()
}

pub fn categories(config: &Value) -> Vec<String> {
    ["tab_set_gen", "tab_set_conv", "tab_set_accel", "tab_set_ui"]
        .iter()
        .map(|key| text(config, key))
        .collect()
}

pub fn value(config: &Value, key: &str) -> String {
    if key == "use_terminal_bg" {
        return text(
            config,
            if boolean(config, key) {
                "bg_option_keep"
            } else {
                "bg_option_solid"
            },
        );
    }
    if config[key].is_boolean() {
        return text(
            config,
            if boolean(config, key) {
                "val_yes"
            } else {
                "val_no"
            },
        );
    }
    match key {
        "language" => match string(config, key).as_str() {
            "ru" | "ru_ru" => "Русский",
            "en-US" | "en_us" => "English (US)",
            _ => "English (UK)",
        }
        .into(),
        "cookies" => match string(config, "cookies_mode").as_str() {
            "file" => config::user_path(&string(config, "cookies_file"))
                .file_name()
                .unwrap_or_default()
                .to_string_lossy()
                .into_owned(),
            "browser" => string(config, "cookies_browser"),
            _ => text(config, "cookies_mode_none"),
        },
        "proxy" | "proxy_mode" => text(config, &format!("proxy_{}", string(config, "proxy_mode"))),
        "cookies_mode" => text(config, &format!("cookies_mode_{}", string(config, key))),
        "transcode_mode" => text(
            config,
            if config[key] == "external" {
                "transcode_external"
            } else {
                "transcode_embedded"
            },
        ),
        "theme" | "video_preset" => {
            let items = if key == "theme" {
                config::metadata()["themes"].clone()
            } else {
                config::video_presets()
            };
            let name = if string(config, "language").starts_with("ru") {
                "name_ru"
            } else {
                "name_en"
            };
            items
                .as_array()
                .unwrap()
                .iter()
                .find(|item| item["id"] == config[key])
                .and_then(|item| item[name].as_str())
                .map(str::to_owned)
                .unwrap_or_else(|| string(config, key))
        }
        "thumbnail_format" => string(config, key).to_uppercase(),
        "logo_mode" => match string(config, key).as_str() {
            "ascii" => "ASCII".into(),
            "image" => text(config, "logo_mode_img"),
            _ => text(config, "val_no"),
        },
        "logo_protocol" => text(config, &format!("logo_proto_{}", string(config, key))),
        "logo_image_path" => {
            let path = string(config, key);
            if path.is_empty() {
                "(none)".into()
            } else {
                config::user_path(&path)
                    .file_name()
                    .unwrap_or_default()
                    .to_string_lossy()
                    .into_owned()
            }
        }
        "reset" => String::new(),
        _ => string(config, key),
    }
}

fn item(config: &Value, key: &'static str, label: &str, tag: &'static str) -> Setting {
    let label = text(config, label);
    let label = label
        .trim_end_matches("%s")
        .trim_end_matches("%d")
        .trim_end_matches([' ', ':'])
        .to_string();
    Setting {
        key,
        label,
        value: value(config, key),
        tag,
    }
}

pub fn rows(category: usize, config: &Value) -> Vec<Setting> {
    match category {
        0 => vec![
            item(config, "download_dir", "settings_download_dir", "-o"),
            item(config, "language", "settings_language", "i18n"),
            item(config, "cookies", "settings_cookies", "--cookies"),
            item(config, "proxy", "settings_proxy", "--proxy"),
            item(
                config,
                "use_archive",
                "settings_archive",
                "--download-archive",
            ),
        ],
        1 => vec![
            item(config, "video_preset", "settings_preset", "--recode-video"),
            item(
                config,
                "transcode_mode",
                "settings_transcode_mode",
                "ffmpeg standalone",
            ),
            item(
                config,
                "audio_format",
                "settings_audio_format",
                "--audio-format",
            ),
            item(config, "sub_langs", "settings_sub_langs", "--sub-langs"),
            item(
                config,
                "thumbnail_format",
                "settings_thumb_fmt",
                "--convert-thumbnails",
            ),
            item(
                config,
                "use_ffmpeg_suffix",
                "settings_ffmpeg_suffix",
                "clean_names",
            ),
            item(
                config,
                "overwrite_original",
                "settings_overwrite_orig",
                "replace_src",
            ),
        ],
        2 => vec![
            item(
                config,
                "concurrent_fragments",
                "settings_accel_frags",
                "--concurrent-fragments",
            ),
            item(config, "no_mtime", "settings_no_mtime", "--no-mtime"),
            item(
                config,
                "windows_filenames",
                "settings_win_names",
                "--windows-filenames",
            ),
        ],
        _ => {
            let mut rows = vec![
                item(config, "theme", "settings_theme", "color_scheme"),
                item(config, "use_terminal_bg", "settings_bg", "transparency"),
                Setting {
                    key: "use_terminal_palette",
                    label: if string(config, "language").starts_with("ru") {
                        "Палитра цветов терминала"
                    } else {
                        "Terminal color palette"
                    }
                    .into(),
                    value: value(config, "use_terminal_palette"),
                    tag: "ANSI",
                },
                item(config, "progress_style", "settings_style", "style"),
                item(
                    config,
                    "bg_queue_max",
                    "settings_bg_queue_max",
                    "queue_slots",
                ),
                item(config, "logo_mode", "settings_logo_mode", "logo"),
            ];
            if config["logo_mode"] == "ascii" {
                rows.push(item(
                    config,
                    "logo_ascii_preset",
                    "settings_logo_ascii",
                    "ascii",
                ));
            } else if config["logo_mode"] == "image" {
                rows.push(item(
                    config,
                    "logo_protocol",
                    "settings_logo_protocol",
                    "protocol",
                ));
                rows.push(item(
                    config,
                    "logo_image_path",
                    "settings_logo_image",
                    "image",
                ));
            }
            rows.push(item(config, "reset", "settings_reset", "factory_wipe"));
            rows
        }
    }
}

pub fn edit_keys(key: &str) -> Vec<&str> {
    match key {
        "cookies" => vec!["cookies_mode", "cookies_browser", "cookies_file"],
        "proxy" => vec!["proxy_mode", "proxy_url"],
        _ => vec![key],
    }
}

pub fn label(config: &Value, key: &str) -> String {
    match key {
        "cookies_mode" => text(config, "cookiesMode"),
        "cookies_browser" => text(config, "cookiesBrowser"),
        "cookies_file" => text(config, "cookiesFile"),
        "proxy_mode" => text(config, "proxy"),
        "proxy_url" => text(config, "proxyUrl"),
        _ => (0..4)
            .flat_map(|category| rows(category, config))
            .find(|row| row.key == key)
            .map(|row| row.label)
            .unwrap_or_else(|| key.to_owned()),
    }
    .trim_end_matches(':')
    .to_string()
}

pub fn draw(frame: &mut Frame, area: Rect, config: &Value, navigation: &mut Navigation) {
    let colors = crate::theme::colors(config);
    let base = Style::default().fg(colors.foreground).bg(colors.background);
    let selected = base
        .fg(colors.accent)
        .add_modifier(Modifier::BOLD | Modifier::REVERSED);
    let sidebar_width = 28.min((area.width / 2).max(10));
    let columns = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Length(sidebar_width), Constraint::Min(1)])
        .split(area);
    let sidebar = Block::default()
        .borders(Borders::RIGHT)
        .style(base.fg(colors.muted));
    let sidebar_inner = sidebar.inner(columns[0]);
    frame.render_widget(sidebar, columns[0]);
    let sidebar_content = Rect::new(
        sidebar_inner.x + 1,
        sidebar_inner.y + 1,
        sidebar_inner.width.saturating_sub(2),
        sidebar_inner.height.saturating_sub(3),
    );
    let items: Vec<_> = categories(config).into_iter().map(ListItem::new).collect();
    navigation.category = navigation.category.min(3);
    frame.render_stateful_widget(
        List::new(items)
            .style(base)
            .highlight_style(if navigation.in_values {
                base.fg(colors.accent).add_modifier(Modifier::BOLD)
            } else {
                selected
            })
            .highlight_symbol("► "),
        sidebar_content,
        &mut ListState::default().with_selected(Some(navigation.category)),
    );
    if sidebar_inner.height >= 2 {
        frame.render_widget(
            Paragraph::new(crate::VERSION).style(base.fg(colors.muted)),
            Rect::new(
                sidebar_inner.x + 2,
                sidebar_inner.bottom() - 2,
                sidebar_inner.width.saturating_sub(3),
                1,
            ),
        );
    }
    let content = Rect::new(
        columns[1].x + 2,
        columns[1].y + 1,
        columns[1].width.saturating_sub(4),
        columns[1].height.saturating_sub(2),
    );
    let settings = rows(navigation.category, config);
    navigation.selected = navigation.selected.min(settings.len().saturating_sub(1));
    let show_tags = content.width >= 110;
    let label_width = (content.width * 55 / 100).clamp(12, 45);
    let mut widths = vec![Constraint::Length(label_width), Constraint::Min(8)];
    if show_tags {
        widths.push(Constraint::Length(25));
    }
    let table_rows: Vec<_> = settings
        .iter()
        .map(|setting| {
            let mut cells = vec![
                Cell::from(setting.label.clone()),
                Cell::from(setting.value.clone()).style(base.fg(colors.accent)),
            ];
            if show_tags {
                cells.push(Cell::from(format!("[{}]", setting.tag)).style(base.fg(colors.muted)));
            }
            Row::new(cells)
        })
        .collect();
    let russian = string(config, "language").starts_with("ru");
    let mut headers = vec![
        if russian {
            "Настройка"
        } else {
            "Setting"
        },
        if russian { "Значение" } else { "Value" },
    ];
    if show_tags {
        headers.push("");
    }
    let table = Table::new(table_rows, widths)
        .column_spacing(2)
        .header(
            Row::new(headers)
                .style(base.fg(colors.muted))
                .bottom_margin(1),
        )
        .row_highlight_style(if navigation.in_values { selected } else { base })
        .highlight_symbol(if navigation.in_values { "> " } else { "  " });
    frame.render_stateful_widget(
        table,
        content,
        &mut TableState::default().with_selected(Some(navigation.selected)),
    );
}
