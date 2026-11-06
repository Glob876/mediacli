use crate::config::{boolean, string};
use ratatui::style::Color;
use serde_json::Value;

#[derive(Clone, Copy)]
pub struct Colors {
    pub accent: Color,
    pub foreground: Color,
    pub background: Color,
    pub muted: Color,
}

/// Indexed colors resolve through the terminal's own configured ANSI palette.
pub fn colors(config: &Value) -> Colors {
    let theme = string(config, "theme");
    let terminal_palette = boolean(config, "use_terminal_palette") || theme == "terminal";
    if terminal_palette {
        let accent = match theme.as_str() {
            "matrix" | "everforest" => 2,
            "fire" | "rose_pine" => 1,
            "gruvbox" | "amber" | "solarized_light" => 3,
            "dracula" | "purple" | "catppuccin" => 5,
            "tokyo_night" | "one_dark" => 4,
            "classic" => 7,
            _ => 6,
        };
        return Colors {
            accent: Color::Indexed(accent),
            foreground: Color::Reset,
            background: if boolean(config, "use_terminal_bg") {
                Color::Reset
            } else {
                Color::Indexed(0)
            },
            muted: Color::Indexed(8),
        };
    }
    let (accent, background, foreground) = match theme.as_str() {
        "nord" => (0x88c0d0, 0x2e3440, 0xeceff4),
        "matrix" => (0x00ff87, 0x0a100c, 0xc7f9cc),
        "dracula" => (0xbd93f9, 0x282a36, 0xf8f8f2),
        "gruvbox" => (0xfabd2f, 0x282828, 0xebdbb2),
        "fire" => (0xff6b6b, 0x1f1212, 0xffe5df),
        "classic" => (0xffffff, 0x101010, 0xeeeeee),
        "solarized_dark" => (0x2aa198, 0x002b36, 0x93a1a1),
        "solarized_light" => (0x268bd2, 0xfdf6e3, 0x586e75),
        "tokyo_night" => (0x7aa2f7, 0x1a1b26, 0xc0caf5),
        "catppuccin" => (0xcba6f7, 0x1e1e2e, 0xcdd6f4),
        "one_dark" => (0x61afef, 0x282c34, 0xabb2bf),
        "monokai" => (0xa6e22e, 0x272822, 0xf8f8f2),
        "rose_pine" => (0xebbcba, 0x191724, 0xe0def4),
        "everforest" => (0xa7c080, 0x2d353b, 0xd3c6aa),
        "amber" => (0xffbf00, 0x19150b, 0xf4e5bc),
        "purple" => (0xc792ea, 0x21172e, 0xe7d9f5),
        _ => (0x00d7ff, 0x101218, 0xe4e7ef),
    };
    Colors {
        accent: rgb(accent),
        foreground: rgb(foreground),
        background: if boolean(config, "use_terminal_bg") {
            Color::Reset
        } else {
            rgb(background)
        },
        muted: Color::DarkGray,
    }
}

fn rgb(value: u32) -> Color {
    Color::Rgb((value >> 16) as u8, (value >> 8) as u8, value as u8)
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn terminal_palette_never_emits_rgb_colors() {
        let mut config = crate::config::defaults();
        config["use_terminal_palette"] = true.into();
        config["use_terminal_bg"] = false.into();
        for theme in crate::config::metadata()["themes"].as_array().unwrap() {
            config["theme"] = theme["id"].clone();
            let palette = colors(&config);
            for color in [
                palette.accent,
                palette.foreground,
                palette.background,
                palette.muted,
            ] {
                assert!(!matches!(color, Color::Rgb(..)));
            }
            crate::config::validate(&config).unwrap();
        }
    }
}
