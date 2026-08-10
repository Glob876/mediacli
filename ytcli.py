"""
MogDop's MediaCLI — a comprehensive console wrapper around yt-dlp & FFmpeg
for Linux, macOS, and Windows with a navigable tabbed TUI menu.

Default UI language is English. Russian can be turned on in Settings -> Language.

Features:
  - Tabbed Navigation for Main Menu & Settings (Switch tabs with Left/Right arrows).
  - Built-in Proxy Support (HTTP/HTTPS/SOCKS5).
  - Dynamic Multi-Stage Process Tracker (displays ONLY active command stages).
  - Formatted Progress Bar Styles (Blocks, Classic, Dots, Minimal).
  - Auto-installation of dependencies on Arch Linux.
  - Polite, formal Russian localization ("Вы / Введите / Выберите").
  - Advanced YouTube Video Downloading:
      * Multi-audio / All available audio tracks downloading & embedding.
      * Video Codec Preference (Force AV1, VP9, H.264, or Auto).
      * Geo-Bypass & Custom Proxy integration.
      * Bandwidth Rate Limiting (2MB/s, 5MB/s, 10MB/s, Unlimited).
      * SponsorBlock integration (auto-cut sponsors & self-promos).
      * Time clipping / section downloading (*00:01:00-00:03:00).
      * Live Stream Mode (--live-from-start).
      * Save description & thumbnail as separate files.
      * Embed Thumbnails, Metadata, Chapters & Subtitles.
      * FPS caps (30 FPS / 60 FPS / Max).
  - Local FFmpeg Tools: Convert, Batch Convert, Trim & FFprobe Inspector.
  - First-time setup wizard & Operation History tracking.
  - Ctrl+C (SIGINT) is disabled. Quit via Ctrl+Q or Exit menu item.

Scriptable CLI usage:
  ./mediacli.py video <url> [-q 1080] [-p davinci-dnxhr] [--proxy socks5://127.0.0.1:1080]
  ./mediacli.py convert <file_path> [-p davinci_dnxhr_hq] [-o DIR]
  ./mediacli.py trim <file_path> -s 00:01:00 -e 00:02:30 [--copy]
  ./mediacli.py probe <file_path>
  ./mediacli.py audio <url> [-f wav] [-o DIR]
  ./mediacli.py playlist <url> [-o DIR] [--audio-only]
  ./mediacli.py info <url>
  ./mediacli.py subs <url> [-l ru,en] [-o DIR]
  ./mediacli.py update
  ./mediacli.py            # TUI menu
"""

import argparse
import json
import locale
import os
import re
import select
import shutil
import signal
import subprocess
import sys
import time
from pathlib import Path

# Safe curses import for Windows compatibility
try:
    import curses
except ImportError:
    curses = None

# ==========================================================================
# Cross-Platform Directory Resolver
# ==========================================================================

def get_config_dir() -> Path:
    if sys.platform == "win32":
        appdata = os.environ.get("APPDATA")
        if appdata:
            return Path(appdata) / "mediacli"
        return Path.home() / "AppData" / "Roaming" / "mediacli"
    elif sys.platform == "darwin":
        return Path.home() / "Library" / "Application Support" / "mediacli"
    else:
        xdg = os.environ.get("XDG_CONFIG_HOME")
        if xdg:
            return Path(xdg) / "mediacli"
        return Path.home() / ".config" / "mediacli"


CONFIG_DIR = get_config_dir()
CONFIG_PATH = CONFIG_DIR / "config.json"
HISTORY_PATH = CONFIG_DIR / "history.json"
DEFAULT_COOKIES_FILE = str(CONFIG_DIR / "cookies.txt")
DEFAULT_DOWNLOAD_DIR = str(Path.home() / "Downloads" / "MediaCLI")

DEFAULT_CONFIG = {
    "first_run_completed": False,
    "download_dir": DEFAULT_DOWNLOAD_DIR,
    "audio_format": "mp3",
    "sub_langs": "ru,en",
    "language": "en",             # "en" or "ru"
    "user_goal": "editing",       # "editing" | "downloading" | "audio" | "transcoding"
    "cookies_mode": "none",       # "none" | "file" | "browser"
    "cookies_file": DEFAULT_COOKIES_FILE,
    "cookies_browser": "",        # e.g. "firefox" or "chrome:Default"
    "video_preset": "davinci_dnxhr",
    "theme": "cyan",              # "cyan" | "nord" | "matrix" | "dracula" | "gruvbox" | "fire" | "classic"
    "use_terminal_bg": True,      # Keep terminal default background (transparent)
    "proxy": "",                  # Built-in proxy (e.g. socks5://127.0.0.1:1080)
    "progress_style": "blocks",   # "blocks" | "classic" | "dots" | "minimal"
}

BROWSERS = ["chrome", "chromium", "firefox", "brave", "edge", "opera", "vivaldi", "safari"]
AUDIO_FORMATS = ["mp3", "wav", "m4a", "opus", "flac"]

ASCII_ART = [
    " _____ ______   _______   ________  ___  ________  ________  ___       ___     ",
    "|\\   _ \\  _   \\|\\  ___ \\ |\\   ___ \\|\\  \\|\\   __  \\|\\   ____\\|\\  \\     |\\  \\    ",
    "\\ \\  \\\\\\__\\ \\  \\ \\   __/|\\ \\  \\_|\\ \\ \\  \\ \\  \\|\\  \\ \\  \\___|\\ \\  \\    \\ \\  \\   ",
    " \\ \\  \\\\|__| \\  \\ \\  \\_|/_\\ \\  \\ \\\\ \\ \\  \\ \\   __  \\ \\  \\    \\ \\  \\    \\ \\  \\  ",
    "  \\ \\  \\    \\ \\  \\ \\  \\_|\\ \\ \\  \\_\\\\ \\ \\  \\ \\  \\ \\  \\ \\  \\____\\ \\  \\____\\ \\  \\ ",
    "   \\ \\__\\    \\ \\__\\ \\_______\\ \\_______\\ \\__\\ \\__\\ \\__\\ \\______\\ \\_______\\ \\__\\",
    "    \\|__|     \\|__|\\|_______|\\|_______|\\|__|\\|__|\\|__|\\|_______|\\|_______|\\|__|",
]

# ==========================================================================
# Themes System
# ==========================================================================

THEMES = {
    "cyan": {
        "id": "cyan",
        "name_en": "Arch Cyan (Default)",
        "name_ru": "Arch Cyan (По умолчанию)",
        "hl_fg": 0 if curses else 0,
        "hl_bg": 6 if curses else 6,  # COLOR_CYAN
        "bg": 0 if curses else 0
    },
    "nord": {
        "id": "nord",
        "name_en": "Nord Blue",
        "name_ru": "Nord Blue",
        "hl_fg": 0 if curses else 0,
        "hl_bg": 4 if curses else 4,  # COLOR_BLUE
        "bg": 0 if curses else 0
    },
    "matrix": {
        "id": "matrix",
        "name_en": "Matrix Green",
        "name_ru": "Matrix Green",
        "hl_fg": 0 if curses else 0,
        "hl_bg": 2 if curses else 2,  # COLOR_GREEN
        "bg": 0 if curses else 0
    },
    "dracula": {
        "id": "dracula",
        "name_en": "Dracula Magenta",
        "name_ru": "Dracula Magenta",
        "hl_fg": 0 if curses else 0,
        "hl_bg": 5 if curses else 5,  # COLOR_MAGENTA
        "bg": 0 if curses else 0
    },
    "gruvbox": {
        "id": "gruvbox",
        "name_en": "Gruvbox Yellow",
        "name_ru": "Gruvbox Yellow",
        "hl_fg": 0 if curses else 0,
        "hl_bg": 3 if curses else 3,  # COLOR_YELLOW
        "bg": 0 if curses else 0
    },
    "fire": {
        "id": "fire",
        "name_en": "Fire Red",
        "name_ru": "Fire Red",
        "hl_fg": 0 if curses else 0,
        "hl_bg": 1 if curses else 1,  # COLOR_RED
        "bg": 0 if curses else 0
    },
    "classic": {
        "id": "classic",
        "name_en": "Classic High Contrast (White)",
        "name_ru": "Классический высококонтрастный (Белый)",
        "hl_fg": 0 if curses else 0,
        "hl_bg": 7 if curses else 7,  # COLOR_WHITE
        "bg": 0 if curses else 0
    }
}

HL_ATTR = None


def apply_theme(cfg: dict) -> None:
    global HL_ATTR
    if curses is None:
        return
    try:
        curses.start_color()
        curses.use_default_colors()
        theme_id = cfg.get("theme", "cyan")

        theme = THEMES.get(theme_id, THEMES["cyan"])
        hl_fg = theme["hl_fg"]
        hl_bg = theme["hl_bg"]

        curses.init_pair(1, hl_fg, hl_bg)
        HL_ATTR = curses.color_pair(1)
    except curses.error:
        HL_ATTR = curses.A_REVERSE


# ==========================================================================
# History Tracking Helpers
# ==========================================================================

def add_history_entry(op_type: str, source: str, target: str, status: str = "Done") -> None:
    HISTORY_PATH.parent.mkdir(parents=True, exist_ok=True)
    entries = []
    if HISTORY_PATH.exists():
        try:
            entries = json.loads(HISTORY_PATH.read_text(encoding="utf-8"))
        except Exception:
            entries = []
    entry = {
        "time": time.strftime("%Y-%m-%d %H:%M:%S"),
        "type": op_type,
        "source": source,
        "target": target,
        "status": status
    }
    entries.insert(0, entry)
    entries = entries[:50]  # Keep last 50 entries
    HISTORY_PATH.write_text(json.dumps(entries, indent=2, ensure_ascii=False), encoding="utf-8")


# ==========================================================================
# FFmpeg Download Presets (for yt-dlp downloads)
# ==========================================================================

VIDEO_PRESETS = {
    "davinci_dnxhr": {
        "id": "davinci_dnxhr",
        "name_en": "DaVinci Resolve: DNxHR HQ + PCM Audio (.mov) [RECOMMENDED]",
        "name_ru": "DaVinci Resolve: DNxHR HQ + PCM Аудио (.mov) [РЕКОМЕНДУЕТСЯ]",
        "desc_en": "Converts video to Avid DNxHR HQ with PCM 16-bit audio. Optimal for DaVinci Resolve (especially Free version).",
        "desc_ru": "Конвертирует в Avid DNxHR HQ с PCM 16-бит аудио. Идеально для бесплатной версии DaVinci Resolve.",
        "args": ["--recode-video", "mov", "--postprocessor-args", "ffmpeg:-c:v dnxhd -profile:v dnxhr_hq -c:a pcm_s16le"]
    },
    "davinci_prores": {
        "id": "davinci_prores",
        "name_en": "DaVinci Resolve: Apple ProRes HQ + PCM Audio (.mov)",
        "name_ru": "DaVinci Resolve: Apple ProRes HQ + PCM Аудио (.mov)",
        "desc_en": "Apple ProRes HQ video codec with uncompressed PCM audio in MOV container.",
        "desc_ru": "Видеокодек Apple ProRes HQ с несжатым PCM аудио в контейнере MOV.",
        "args": ["--recode-video", "mov", "--postprocessor-args", "ffmpeg:-c:v prores_ks -profile:v 3 -c:a pcm_s16le"]
    },
    "davinci_h264": {
        "id": "davinci_h264",
        "name_en": "DaVinci Resolve: H.264 + PCM Audio (.mov)",
        "name_ru": "DaVinci Resolve: H.264 + PCM Аудио (.mov)",
        "desc_en": "H.264 video with PCM audio in MOV container. Compact size while fixing silent audio in DaVinci.",
        "desc_ru": "Видео H.264 с PCM аудио в MOV контейнере. Компактный размер и рабочий звук в DaVinci.",
        "args": ["--recode-video", "mov", "--postprocessor-args", "ffmpeg:-c:v libx264 -pix_fmt yuv420p -c:a pcm_s16le"]
    },
    "standard_mp4": {
        "id": "standard_mp4",
        "name_en": "Standard MP4 (H.264 + AAC)",
        "name_ru": "Стандартный MP4 (H.264 + AAC)",
        "desc_en": "Universal MP4 container with H.264 video and AAC audio. High compatibility for web and media players.",
        "desc_ru": "Универсальный MP4 с H.264 видео и AAC аудио. Максимальная совместимость с плеерами и соцсетями.",
        "args": ["--recode-video", "mp4", "--postprocessor-args", "ffmpeg:-c:v libx264 -c:a aac"]
    },
    "default": {
        "id": "default",
        "name_en": "Original (yt-dlp auto merge)",
        "name_ru": "Исходный (авто-объединение yt-dlp)",
        "desc_en": "Merges best available streams into MP4 without re-encoding video.",
        "desc_ru": "Объединяет наилучшие исходные потоки без перекодирования видео.",
        "args": ["--merge-output-format", "mp4"]
    },
    "custom": {
        "id": "custom",
        "name_en": "Custom FFmpeg preset...",
        "name_ru": "Свой пресет FFmpeg...",
        "desc_en": "Manually specify container extension and custom FFmpeg arguments.",
        "desc_ru": "Указать расширение файла и собственные аргументы FFmpeg вручную.",
        "args": []
    }
}

PRESET_CLI_MAP = {
    "default": "default",
    "davinci-dnxhr": "davinci_dnxhr",
    "davinci-prores": "davinci_prores",
    "davinci-h264": "davinci_h264",
    "standard-mp4": "standard_mp4",
    "custom": "custom",
}

# ==========================================================================
# Local File Conversion Presets (for FFmpeg converter tab)
# ==========================================================================

CONVERT_PRESETS = [
    {
        "id": "davinci_dnxhr_hq",
        "name_en": "DaVinci Resolve: DNxHR HQ + PCM (.mov) [Recommended]",
        "name_ru": "DaVinci Resolve: DNxHR HQ + PCM (.mov) [Рекомендуется]",
        "desc_en": "Avid DNxHR HQ video codec with uncompressed 16-bit PCM audio in MOV container. Works reliably in DaVinci Resolve.",
        "desc_ru": "Кодек Avid DNxHR HQ с несжатым PCM 16-бит аудио в MOV. Надёжно работает в бесплатном DaVinci.",
        "ext": "mov",
        "suffix": "_dnxhr",
        "ffmpeg_flags": ["-c:v", "dnxhd", "-profile:v", "dnxhr_hq", "-c:a", "pcm_s16le"]
    },
    {
        "id": "davinci_prores",
        "name_en": "DaVinci Resolve: Apple ProRes HQ + PCM (.mov)",
        "name_ru": "DaVinci Resolve: Apple ProRes HQ + PCM (.mov)",
        "desc_en": "Apple ProRes HQ video codec with PCM 16-bit audio. Studio quality for video editing.",
        "desc_ru": "Видеокодек Apple ProRes HQ с несжатым PCM 16-бит аудио. Студийный стандарт для видеомонтажа.",
        "ext": "mov",
        "suffix": "_prores",
        "ffmpeg_flags": ["-c:v", "prores_ks", "-profile:v", "3", "-c:a", "pcm_s16le"]
    },
    {
        "id": "davinci_h264",
        "name_en": "DaVinci Resolve: H.264 + PCM Audio (.mov)",
        "name_ru": "DaVinci Resolve: H.264 + PCM Аудио (.mov)",
        "desc_en": "H.264 video with PCM audio in MOV container. Smaller file size while maintaining audio compatibility in DaVinci.",
        "desc_ru": "Видео H.264 с PCM аудио в контейнере MOV. Меньший размер файла и рабочий звук в DaVinci.",
        "ext": "mov",
        "suffix": "_davinci",
        "ffmpeg_flags": ["-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "pcm_s16le"]
    },
    {
        "id": "standard_mp4",
        "name_en": "Standard MP4 (H.264 + AAC)",
        "name_ru": "Стандартный MP4 (H.264 + AAC)",
        "desc_en": "Universal MP4 container with H.264 video and AAC audio. Compatible with all smartphones, web, and TVs.",
        "desc_ru": "Универсальный MP4 с H.264 видео и AAC аудио. Совместим со всеми смартфонами, сайтами и ТВ.",
        "ext": "mp4",
        "suffix": "_mp4",
        "ffmpeg_flags": ["-c:v", "libx264", "-c:a", "aac", "-b:a", "192k"]
    },
    {
        "id": "audio_wav",
        "name_en": "Extract Audio: Uncompressed WAV (.wav)",
        "name_ru": "Извлечь аудио: WAV без сжатия (.wav)",
        "desc_en": "Extracts audio track into uncompressed 16-bit PCM WAV file.",
        "desc_ru": "Извлекает аудиодорожку в несжатый WAV файл (PCM 16-бит).",
        "ext": "wav",
        "suffix": "_audio",
        "ffmpeg_flags": ["-vn", "-acodec", "pcm_s16le"]
    },
    {
        "id": "audio_mp3",
        "name_en": "Extract Audio: MP3 320kbps (.mp3)",
        "name_ru": "Извлечь аудио: MP3 320 кбит/с (.mp3)",
        "desc_en": "Extracts audio track into high-bitrate MP3 audio file.",
        "desc_ru": "Извлекает аудиодорожку в высококачественный MP3 файл 320 кбит/с.",
        "ext": "mp3",
        "suffix": "_audio",
        "ffmpeg_flags": ["-vn", "-ab", "320k"]
    },
    {
        "id": "custom",
        "name_en": "Custom FFmpeg command...",
        "name_ru": "Свой пресет FFmpeg...",
        "desc_en": "Specify custom file extension and custom FFmpeg flags manually.",
        "desc_ru": "Задать собственное расширение файла и произвольные флаги FFmpeg вручную.",
        "ext": "mov",
        "suffix": "_custom",
        "ffmpeg_flags": []
    }
]


def get_ordered_video_preset_keys(cfg: dict) -> list[str]:
    goal = cfg.get("user_goal", "editing")
    if goal == "downloading":
        return ["standard_mp4", "default", "davinci_dnxhr", "davinci_prores", "davinci_h264", "custom"]
    elif goal == "audio":
        return ["standard_mp4", "default", "davinci_dnxhr", "custom"]
    else:  # "editing" or "transcoding"
        return ["davinci_dnxhr", "davinci_prores", "davinci_h264", "standard_mp4", "default", "custom"]


def get_ordered_convert_presets(cfg: dict) -> list[dict]:
    goal = cfg.get("user_goal", "editing")
    if goal == "downloading" or goal == "transcoding":
        order = ["standard_mp4", "davinci_dnxhr_hq", "davinci_prores", "davinci_h264", "audio_mp3", "audio_wav", "custom"]
    elif goal == "audio":
        order = ["audio_mp3", "audio_wav", "standard_mp4", "davinci_dnxhr_hq", "davinci_prores", "custom"]
    else:  # "editing"
        order = ["davinci_dnxhr_hq", "davinci_prores", "davinci_h264", "audio_wav", "standard_mp4", "audio_mp3", "custom"]

    preset_dict = {p["id"]: p for p in CONVERT_PRESETS}
    return [preset_dict[pid] for pid in order if pid in preset_dict]


# ==========================================================================
# Signal Handlers (Disable Ctrl+C)
# ==========================================================================

def setup_signal_handlers() -> None:
    """Ignore SIGINT (Ctrl+C) so the application cannot be killed accidentally."""
    signal.signal(signal.SIGINT, signal.SIG_IGN)


# ==========================================================================
# i18n
# ==========================================================================

STRINGS = {
    "en": {
        "app_title": "MogDop's MediaCLI",
        "tab_online": "Downloads",
        "tab_local": "Local Tools",
        "tab_system": "System & Config",

        "tab_set_gen": "General",
        "tab_set_conv": "Conversion",
        "tab_set_ui": "Interface",

        "menu_video": "Download video (YouTube)",
        "menu_audio": "Download audio",
        "menu_playlist": "Download playlist",
        "menu_convert": "Convert local file (FFmpeg)",
        "menu_batch_convert": "Batch convert folder",
        "menu_trim": "Trim video / audio",
        "menu_probe": "Inspect local file (FFprobe)",
        "menu_info": "Video URL info",
        "menu_subs": "Download subtitles",
        "menu_history": "Operation history",
        "menu_cookies": "Cookies",
        "menu_settings": "Settings",
        "menu_update": "Update yt-dlp",
        "menu_exit": "Exit",

        "footer_nav": "<- / -> switch tabs   up/down or j/k - navigate   Enter - select   Esc/q - back",
        "footer_input": "Enter - confirm   Esc - cancel   Ctrl+Q - quit",
        "footer_message": "Enter/Esc - continue",

        "video_title": "Download video",
        "video_prompt_url": "Paste the video URL:",
        "video_quality_subtitle": "Choose max quality",
        "quality_best": "Best available",
        "quality_custom": "Custom...",
        "quality_custom_prompt": "Enter height in pixels, e.g. 1080:",
        "quality_best_short": "best",
        "preset_subtitle": "Choose FFmpeg / Codec Preset",
        "subs_choice_none": "No subtitles",
        "subs_choice_yes": "Download subtitles",
        "subs_langs_prompt": "Subtitle languages, comma separated:",
        "outdir_prompt": "Destination folder:",
        "confirm_title": "Confirmation",
        "confirm_start": "Start process",
        "confirm_cancel": "Cancel",

        "convert_title": "Convert local file",
        "convert_prompt_file": "Enter full path to input video/audio file:",
        "convert_prompt_preset": "Choose target conversion preset:",
        "convert_prompt_outdir": "Destination folder (leave empty for same folder):",
        "convert_err_notfound": "Error: File/Folder '{f}' does not exist!",

        "batch_title": "Batch Convert Folder",
        "batch_prompt_folder": "Enter path to folder containing media files:",
        "batch_no_files": "No media files (.mp4, .mkv, .mov, .avi, etc.) found in folder.",

        "trim_title": "Trim Media File",
        "trim_prompt_file": "Enter path to video/audio file:",
        "trim_prompt_start": "Start time (HH:MM:SS or seconds, e.g. 00:01:30):",
        "trim_prompt_end": "End time / Duration (HH:MM:SS or seconds, e.g. 00:02:45):",
        "trim_mode_copy": "Fast Stream Copy (-c copy, instant, no quality loss)",
        "trim_mode_reencode": "Re-encode using target preset",

        "probe_title": "Inspect Local File (FFprobe)",
        "probe_prompt_file": "Enter path to media file:",

        "history_title": "Operation History",
        "history_empty": "No operations recorded yet.",

        "custom_ext_prompt": "Target file extension (e.g. mp4, mov, mkv, avi):",
        "custom_flags_prompt": "Custom FFmpeg flags (e.g. -c:v libx264 -c:a pcm_s16le):",

        # Advanced Video Options strings
        "adv_title": "Advanced Download Settings",
        "adv_audio_track": "Audio Track(s): {v}",
        "adv_audio_default": "Default Track",
        "adv_audio_all": "ALL Available Tracks (Multilingual)",
        "adv_audio_ru": "Russian (ru)",
        "adv_audio_en": "English (en)",
        "adv_audio_custom": "Custom Lang Code",
        "adv_audio_prompt": "Enter audio language code (e.g. es, ja, de):",
        "adv_vcodec": "Video Codec Preference: {v}",
        "adv_vcodec_auto": "Auto Best",
        "adv_vcodec_av1": "Force AV1",
        "adv_vcodec_vp9": "Force VP9",
        "adv_vcodec_h264": "Force H.264 (AVC)",
        "adv_sponsorblock": "SponsorBlock (Auto-cut Sponsors): {v}",
        "adv_sb_off": "Disabled",
        "adv_sb_sponsors": "Cut Sponsors Only",
        "adv_sb_all": "Cut Sponsors + Promos + Intros",
        "adv_geobypass": "Geo-Bypass (--geo-bypass): {v}",
        "adv_ratelimit": "Download Speed Limit: {v}",
        "adv_ratelimit_max": "Unlimited",
        "adv_clip": "Time Clip / Section: {v}",
        "adv_clip_full": "Full Video",
        "adv_clip_prompt": "Enter time section (e.g. *00:01:00-00:03:30):",
        "adv_live": "Live Stream Mode: {v}",
        "adv_live_default": "Standard",
        "adv_live_start": "Download Live Stream From Start",
        "adv_write_extra": "Save Description & Cover Files: {v}",
        "adv_embed_meta": "Embed Thumbnail & Metadata: {v}",
        "adv_embed_chap": "Embed Video Chapters: {v}",
        "adv_embed_subs": "Embed Subtitles into Video: {v}",
        "adv_fps": "FPS Limit: {v}",
        "adv_fps_max": "Max available",
        "adv_proxy": "Custom Proxy: {v}",
        "adv_proxy_none": "None",
        "adv_proxy_prompt": "Enter proxy URL (e.g. socks5://127.0.0.1:1080):",
        "adv_proceed": "-> Proceed to Download ->",

        "label_url": "URL:",
        "label_input": "Input File:",
        "label_output": "Output File:",
        "label_quality": "Quality:",
        "label_preset": "Codec Preset:",
        "label_folder": "Folder:",
        "label_format": "Format:",
        "label_mode": "Mode:",
        "label_langs": "Languages:",
        "label_cmd": "Command:",

        "audio_title": "Download audio",
        "audio_format_subtitle": "Choose audio format (WAV recommended for DaVinci Resolve)",

        "playlist_title": "Download playlist",
        "playlist_prompt_url": "Paste the playlist URL:",
        "playlist_mode_video": "Video",
        "playlist_mode_audio": "Audio only",

        "info_title": "Video info",
        "info_fetching": "Fetching info...",
        "info_error": "Error fetching info:",
        "info_not_installed": "yt-dlp is not installed.",
        "info_parse_error": "Failed to parse yt-dlp response.",
        "info_label_title": "Title:",
        "info_label_uploader": "Channel:",
        "info_label_duration": "Duration:",
        "info_label_views": "Views:",
        "info_label_date": "Date:",
        "info_label_quality": "Available quality:",

        "subs_title": "Download subtitles",

        "log_title": "Process status",
        "log_footer_running": "F10 - toggle raw logs   q/Esc - cancel process",
        "log_footer_done": "Enter/Esc - back to menu",
        "log_finished_ok": "Done (exit code 0).",
        "log_finished_err": "Finished with exit code {v}.",

        "cookies_title": "Cookies",
        "cookies_status_none": "Not configured (no cookies sent)",
        "cookies_status_file": "File ({v} entries): {f}",
        "cookies_status_browser": "Browser: {v}",
        "cookies_opt_view": "View cookies file",
        "cookies_opt_add": "Add cookie entry",
        "cookies_opt_edit_file": "Edit cookies file in $EDITOR",
        "cookies_opt_set_path": "Change cookies file path",
        "cookies_opt_browser": "Use cookies from browser instead",
        "cookies_opt_disable": "Disable cookies",
        "cookies_opt_back": "Back",
        "cookies_browser_subtitle": "Choose browser",
        "cookies_profile_prompt": "Browser profile (leave empty for default):",
        "cookies_path_prompt": "Path to cookies.txt (Netscape format, used by yt-dlp --cookies):",
        "cookies_domain_prompt": "Cookie domain, e.g. .youtube.com:",
        "cookies_name_prompt": "Cookie name:",
        "cookies_value_prompt": "Cookie value:",
        "cookies_added": "Cookie added to {v}",
        "cookies_none_found": "No cookies found in this file yet.",
        "cookies_view_note": "(this is your yt-dlp --cookies file, Netscape format)",

        "settings_title": "Settings",
        "settings_download_dir": "Download folder: {v}",
        "settings_audio_format": "Default audio format: {v}",
        "settings_sub_langs": "Default subtitle languages: {v}",
        "settings_language": "Language: {v}",
        "settings_goal": "Primary Purpose / Use-case: {v}",
        "settings_preset": "Default Video/FFmpeg Preset: {v}",
        "settings_theme": "Color Theme: {v}",
        "settings_bg": "Terminal Background: {v}",
        "settings_proxy": "Network Proxy: {v}",
        "settings_style": "Progress Bar Style: {v}",
        "settings_cookies": "Cookies: {v}",
        "settings_reset": "Reset settings to defaults...",
        "settings_back": "Back",
        "settings_new_download_dir": "New download folder:",
        "settings_choose_audio_format": "Default audio format",
        "settings_new_sub_langs": "Subtitle languages, comma separated:",
        "settings_choose_language": "Choose language",
        "settings_choose_preset": "Choose default video export preset",
        "settings_choose_goal": "Choose primary usage goal (reorders presets)",
        "settings_choose_theme": "Choose color theme",
        "settings_reset_confirm": "Reset all settings to defaults and re-run setup wizard?",

        "update_title": "Update yt-dlp",
        "update_via_pkg": "Via System Package Manager (pacman/apt/brew/winget)",
        "update_via_pip": "Via Python Pip",
        "update_via_ytdlp": "Via yt-dlp self-update (-U)",
        "update_cancel": "Cancel",

        "deps_title": "Warning",
        "deps_missing_header": "Missing dependencies:",
        "deps_install_header": "Install dependencies on your system:",
        "deps_arch_install_prompt": "Would you like to automatically install missing dependencies via pacman now?",
        "deps_note": "The menu will still open, but downloads/conversions won't work.",

        # Wizard strings
        "wizard_title": "MogDop's MediaCLI Setup Wizard",
        "wizard_step1_title": "Step 1/4: Select Language",
        "wizard_step2_title": "Step 2/4: Primary Goal / Use Case",
        "wizard_step2_subtitle": "This determines default preset choices and ordering across the app",
        "goal_editing": "Video Editing (DaVinci Resolve / Premiere / Video Editors)",
        "goal_downloading": "General Video Downloading (YouTube / MP4)",
        "goal_audio": "Audio & Music Extraction (MP3 / WAV)",
        "goal_transcoding": "Local Media Transcoding / FFmpeg Conversions",
        "wizard_step3_title": "Step 3/4: Default Download Directory",
        "wizard_step4_title": "Step 4/4: Theme & Appearance",
        "bg_option_title": "Terminal Background Mode",
        "bg_option_keep": "Keep terminal default background (Transparent / Native)",
        "bg_option_solid": "Use theme background color (Solid Dark)",
        "wizard_done_title": "Setup Complete!",
        "wizard_done_msg": "Your configuration has been saved. You can change settings anytime in Settings.",
    },
    "ru": {
        "app_title": "MogDop's MediaCLI",
        "tab_online": "Загрузка",
        "tab_local": "Инструменты",
        "tab_system": "Система",

        "tab_set_gen": "Основные",
        "tab_set_conv": "Конвертация",
        "tab_set_ui": "Интерфейс",

        "menu_video": "Скачать видео (YouTube)",
        "menu_audio": "Скачать аудио",
        "menu_playlist": "Скачать плейлист",
        "menu_convert": "Конвертировать локальный файл (FFmpeg)",
        "menu_batch_convert": "Пакетная конвертация папки",
        "menu_trim": "Обрезать видео / аудио",
        "menu_probe": "Анализ файла (FFprobe)",
        "menu_info": "Информация о ссылке",
        "menu_subs": "Скачать субтитры",
        "menu_history": "История операций",
        "menu_cookies": "Cookies",
        "menu_settings": "Настройки",
        "menu_update": "Обновить yt-dlp",
        "menu_exit": "Выход",

        "footer_nav": "<- / -> вкладки   up/down или j/k — навигация   Enter — выбор   Esc/q — назад",
        "footer_input": "Enter — подтвердить   Esc — отмена   Ctrl+Q — выход",
        "footer_message": "Enter/Esc — продолжить",

        "video_title": "Скачать видео",
        "video_prompt_url": "Введите ссылку на видео:",
        "video_quality_subtitle": "Выберите максимальное качество",
        "quality_best": "Лучшее доступное",
        "quality_custom": "Указать вручную...",
        "quality_custom_prompt": "Введите высоту в пикселях, например 1080:",
        "quality_best_short": "лучшее",
        "preset_subtitle": "Выберите пресет FFmpeg / кодеков",
        "subs_choice_none": "Без субтитров",
        "subs_choice_yes": "Скачать субтитры",
        "subs_langs_prompt": "Языки субтитров через запятую:",
        "outdir_prompt": "Папка для сохранения:",
        "confirm_title": "Подтверждение",
        "confirm_start": "Начать процесс",
        "confirm_cancel": "Отмена",

        "convert_title": "Конвертировать локальный файл",
        "convert_prompt_file": "Введите полный путь к исходному файлу:",
        "convert_prompt_preset": "Выберите целевой пресет конвертации:",
        "convert_prompt_outdir": "Папка назначения (пусто — сохранять в ту же папку):",
        "convert_err_notfound": "Ошибка: Файл или папка '{f}' не существует!",

        "batch_title": "Пакетная конвертация папки",
        "batch_prompt_folder": "Введите путь к папке с видео/аудио файлами:",
        "batch_no_files": "В выбранной папке не найдено подходящих медиафайлов.",

        "trim_title": "Ообрезать медиафайл",
        "trim_prompt_file": "Введите путь к видео/аудиофайлу:",
        "trim_prompt_start": "Время начала (ЧЧ:ММ:СС или секунды, например 00:01:30):",
        "trim_prompt_end": "Время окончания / Длительность (ЧЧ:ММ:СС или секунды):",
        "trim_mode_copy": "Быстрое копирование (-c copy, без потери качества)",
        "trim_mode_reencode": "Перекодировать с выбранным пресетом",

        "probe_title": "Анализ медиафайла (FFprobe)",
        "probe_prompt_file": "Введите путь к медиафайлу:",

        "history_title": "История операций",
        "history_empty": "Записей об операциях пока нет.",

        "custom_ext_prompt": "Расширение итогового файла (например, mp4, mov, mkv, avi):",
        "custom_flags_prompt": "Флаги FFmpeg (например, -c:v libx264 -c:a pcm_s16le):",

        # Advanced Video Options strings
        "adv_title": "Расширенные настройки скачивания",
        "adv_audio_track": "Звуковая дорожка(и): {v}",
        "adv_audio_default": "Дорожка по умолчанию",
        "adv_audio_all": "ВСЕ доступные дорожки (Мультиязычный)",
        "adv_audio_ru": "Русская (ru)",
        "adv_audio_en": "Английская (en)",
        "adv_audio_custom": "Указать код языка...",
        "adv_audio_prompt": "Введите код языка звука (например: es, ja, de):",
        "adv_vcodec": "Приоритет видеокодека: {v}",
        "adv_vcodec_auto": "Авто (Лучший)",
        "adv_vcodec_av1": "Принудительно AV1",
        "adv_vcodec_vp9": "Принудительно VP9",
        "adv_vcodec_h264": "Принудительно H.264 (AVC)",
        "adv_sponsorblock": "SponsorBlock (Вырезка рекламы): {v}",
        "adv_sb_off": "Отключено",
        "adv_sb_sponsors": "Вырезать только интеграции",
        "adv_sb_all": "Вырезать интеграции + заставки + интро",
        "adv_geobypass": "Обход гео-ограничений (--geo-bypass): {v}",
        "adv_ratelimit": "Ограничение скорости скачивания: {v}",
        "adv_ratelimit_max": "Без ограничений",
        "adv_clip": "Обрезка фрагмента / Таймкод: {v}",
        "adv_clip_full": "Полное видео",
        "adv_clip_prompt": "Введите интервал (например: *00:01:00-00:03:30):",
        "adv_live": "Прямые эфиры / Стримы: {v}",
        "adv_live_default": "Стандартно",
        "adv_live_start": "Скачивать стрим с самого начала",
        "adv_write_extra": "Сохранить обложку и описание отд. файлами: {v}",
        "adv_embed_meta": "Вшить обложку и метаданные: {v}",
        "adv_embed_chap": "Вшить главы (Chapters): {v}",
        "adv_embed_subs": "Вшить субтитры в видео: {v}",
        "adv_fps": "Ограничение FPS: {v}",
        "adv_fps_max": "Максимальный",
        "adv_proxy": "Встроенный Прокси: {v}",
        "adv_proxy_none": "Не используется",
        "adv_proxy_prompt": "Введите URL прокси (например, socks5://127.0.0.1:1080):",
        "adv_proceed": "-> Перейти к скачиванию ->",

        "label_url": "URL:",
        "label_input": "Исходный файл:",
        "label_output": "Выходной файл:",
        "label_quality": "Качество:",
        "label_preset": "Пресет кодеков:",
        "label_folder": "Папка:",
        "label_format": "Формат:",
        "label_mode": "Режим:",
        "label_langs": "Языки:",
        "label_cmd": "Команда:",

        "audio_title": "Скачать аудио",
        "audio_format_subtitle": "Выберите формат аудио (WAV рекомендуется для DaVinci Resolve)",

        "playlist_title": "Скачать плейлист",
        "playlist_prompt_url": "Введите ссылку на плейлист:",
        "playlist_mode_video": "Видео",
        "playlist_mode_audio": "Только аудио",

        "info_title": "Информация о видео",
        "info_fetching": "Запрос информации...",
        "info_error": "Ошибка при получении информации:",
        "info_not_installed": "yt-dlp не установлен.",
        "info_parse_error": "Не удалось разобрать ответ yt-dlp.",
        "info_label_title": "Название:",
        "info_label_uploader": "Канал:",
        "info_label_duration": "Длительность:",
        "info_label_views": "Просмотры:",
        "info_label_date": "Дата:",
        "info_label_quality": "Качество:",

        "subs_title": "Скачать субтитры",

        "log_title": "Статус процесса",
        "log_footer_running": "F10 — логи вкл/выкл   q/Esc — отмена процесса",
        "log_footer_done": "Enter/Esc — назад в меню",
        "log_finished_ok": "Готово (код выхода 0).",
        "log_finished_err": "Завершено с кодом выхода {v}.",

        "cookies_title": "Cookies",
        "cookies_status_none": "Не настроено (куки не отправляются)",
        "cookies_status_file": "Файл ({v} записей): {f}",
        "cookies_status_browser": "Браузер: {v}",
        "cookies_opt_view": "Просмотреть файл cookies",
        "cookies_opt_add": "Добавить запись cookie",
        "cookies_opt_edit_file": "Редактировать файл в $EDITOR",
        "cookies_opt_set_path": "Изменить путь к файлу cookies",
        "cookies_opt_browser": "Использовать куки из браузера вместо этого",
        "cookies_opt_disable": "Отключить куки",
        "cookies_opt_back": "Назад",
        "cookies_browser_subtitle": "Выберите браузер",
        "cookies_profile_prompt": "Профиль браузера (пусто — по умолчанию):",
        "cookies_path_prompt": "Путь к файлу cookies.txt (формат Netscape, используется yt-dlp --cookies):",
        "cookies_domain_prompt": "Домен куки, например .youtube.com:",
        "cookies_name_prompt": "Имя куки:",
        "cookies_value_prompt": "Значение куки:",
        "cookies_added": "Cookie добавлен в {v}",
        "cookies_none_found": "В этом файле пока нет куки.",
        "cookies_view_note": "(это твой файл yt-dlp --cookies, формат Netscape)",

        "settings_title": "Настройки",
        "settings_download_dir": "Папка загрузок: {v}",
        "settings_audio_format": "Формат аудио по умолчанию: {v}",
        "settings_sub_langs": "Языки субтитров по умолчанию: {v}",
        "settings_language": "Язык: {v}",
        "settings_goal": "Основная цель применения: {v}",
        "settings_preset": "Пресет видео/FFmpeg: {v}",
        "settings_theme": "Цветовая тема: {v}",
        "settings_bg": "Фон терминала: {v}",
        "settings_proxy": "Сетевой прокси: {v}",
        "settings_style": "Стиль индикатора прогресса: {v}",
        "settings_cookies": "Cookies: {v}",
        "settings_reset": "Сбросить настройки к заводским...",
        "settings_back": "Назад",
        "settings_new_download_dir": "Новая папка загрузок:",
        "settings_choose_audio_format": "Формат аудио по умолчанию",
        "settings_new_sub_langs": "Языки субтитров через запятую:",
        "settings_choose_language": "Выберите язык",
        "settings_choose_preset": "Выберите пресет видео для скачивания",
        "settings_choose_goal": "Основная цель (меняет порядок пресетов)",
        "settings_choose_theme": "Выберите цветовую тему",
        "settings_reset_confirm": "Сбросить все настройки к заводским и пройти настройку заново?",

        "update_title": "Обновить yt-dlp",
        "update_via_pkg": "Через системный пакетный менеджер (pacman/apt/brew/winget)",
        "update_via_pip": "Через Python Pip",
        "update_via_ytdlp": "Через встроенный yt-dlp -U",
        "update_cancel": "Отмена",

        "deps_title": "Внимание",
        "deps_missing_header": "Не найдены зависимости:",
        "deps_install_header": "Установка зависимостей в вашей системе:",
        "deps_arch_install_prompt": "Установить отсутствующие зависимости прямо сейчас через pacman?",
        "deps_note": "Меню всё равно откроется, но скачивание/конвертация не будет работать.",

        # Wizard strings
        "wizard_title": "Мастер первичной настройки MogDop's MediaCLI",
        "wizard_step1_title": "Шаг 1/4: Выбор языка",
        "wizard_step2_title": "Шаг 2/4: Основная цель использования",
        "wizard_step2_subtitle": "Это повлияет на приоритет и порядок пресетов во всей программе",
        "goal_editing": "Видеомонтаж (DaVinci Resolve / Premiere / Видеоредакторы)",
        "goal_downloading": "Обычная загрузка видео (YouTube / MP4)",
        "goal_audio": "Извлечение аудио и музыки (MP3 / WAV)",
        "goal_transcoding": "Конвертация локальных медиафайлов (FFmpeg)",
        "wizard_step3_title": "Шаг 3/4: Папка загрузок по умолчанию",
        "wizard_step4_title": "Шаг 4/4: Цветовая тема и внешний вид",
        "bg_option_title": "Фон терминала",
        "bg_option_keep": "Оставить фон терминала (Прозрачный / Системный)",
        "bg_option_solid": "Использовать сплошной фон темы (Тёмный)",
        "wizard_done_title": "Настройка завершена!",
        "wizard_done_msg": "Конфигурация сохранена. Вы всегда можете изменить эти настройки в меню 'Настройки'.",
    },
}


def t(cfg: dict, key: str, **kwargs) -> str:
    lang = cfg.get("language", "en")
    table = STRINGS.get(lang, STRINGS["en"])
    text = table.get(key, STRINGS["en"].get(key, key))
    return text.format(**kwargs) if kwargs else text


def get_preset_name(cfg: dict, preset_id: str) -> str:
    preset = VIDEO_PRESETS.get(preset_id, VIDEO_PRESETS["default"])
    lang = cfg.get("language", "en")
    return preset["name_ru"] if lang == "ru" else preset["name_en"]


def get_install_command(missing_bins: list[str]) -> str:
    pkgs = " ".join(missing_bins)
    if sys.platform == "win32":
        return f"winget install {' '.join(missing_bins)}"
    elif sys.platform == "darwin":
        return f"brew install {pkgs}"
    else:
        if shutil.which("pacman"):
            return f"sudo pacman -S {pkgs}"
        elif shutil.which("apt"):
            return f"sudo apt update && sudo apt install {pkgs}"
        elif shutil.which("dnf"):
            return f"sudo dnf install {pkgs}"
        return f"pip install {pkgs}"


# ==========================================================================
# Config
# ==========================================================================

def load_config() -> dict:
    cfg = dict(DEFAULT_CONFIG)
    if CONFIG_PATH.exists():
        try:
            cfg.update(json.loads(CONFIG_PATH.read_text(encoding="utf-8")))
        except (json.JSONDecodeError, OSError):
            pass
    return cfg


def save_config(cfg: dict) -> None:
    CONFIG_PATH.parent.mkdir(parents=True, exist_ok=True)
    CONFIG_PATH.write_text(json.dumps(cfg, indent=2, ensure_ascii=False), encoding="utf-8")


def reset_config() -> dict:
    if CONFIG_PATH.exists():
        try:
            CONFIG_PATH.unlink()
        except OSError:
            pass
    return dict(DEFAULT_CONFIG)


def cookie_args(cfg: dict) -> list[str]:
    mode = cfg.get("cookies_mode", "none")
    if mode == "file" and cfg.get("cookies_file"):
        cpath = Path(cfg["cookies_file"]).expanduser()
        if cpath.exists():
            return ["--cookies", str(cpath)]
    if mode == "browser" and cfg.get("cookies_browser"):
        return ["--cookies-from-browser", cfg["cookies_browser"]]
    return []


# ==========================================================================
# Cookies file helpers
# ==========================================================================

def parse_cookies_file(path: str) -> list[dict]:
    p = Path(path).expanduser()
    if not p.exists():
        return []
    cookies = []
    for line in p.read_text(errors="replace").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split("\t")
        if len(parts) != 7:
            continue
        domain, flag, cpath, secure, expiration, name, value = parts
        cookies.append({
            "domain": domain, "flag": flag, "path": cpath, "secure": secure,
            "expiration": expiration, "name": name, "value": value,
        })
    return cookies


def append_cookie_entry(path: str, domain: str, name: str, value: str) -> None:
    p = Path(path).expanduser()
    p.parent.mkdir(parents=True, exist_ok=True)
    if not p.exists():
        p.write_text(
            "# Netscape HTTP Cookie File\n"
            "# Generated by MogDop's MediaCLI, used by yt-dlp's --cookies option.\n\n"
        )
    flag = "TRUE" if domain.startswith(".") else "FALSE"
    expiration = str(int(time.time()) + 10 * 365 * 24 * 3600)
    line = "\t".join([domain, flag, "/", "FALSE", expiration, name, value])
    with p.open("a", encoding="utf-8") as f:
        f.write(line + "\n")


# ==========================================================================
# Dependency check
# ==========================================================================

def missing_dependencies() -> list[tuple[str, str]]:
    deps = [("yt-dlp", "yt-dlp"), ("ffmpeg", "ffmpeg"), ("ffprobe", "ffmpeg")]
    return [(bin_, pkg) for bin_, pkg in deps if not shutil.which(bin_)]


# ==========================================================================
# Low-level curses widgets with Tabbed Navigation
# ==========================================================================

def safe_addstr(stdscr, y: int, x: int, text: str, max_w: int, attr=curses.A_NORMAL) -> None:
    if stdscr is None or max_w <= 0 or y < 0 or x < 0:
        return
    try:
        stdscr.addstr(y, x, text[:max_w], attr)
    except curses.error:
        pass


def draw_header(stdscr, title: str, w: int) -> None:
    safe_addstr(stdscr, 0, 2, title, w - 4, curses.A_BOLD)
    safe_addstr(stdscr, 1, 0, "-" * w, w)


def draw_footer(stdscr, text: str, w: int, h: int) -> None:
    safe_addstr(stdscr, h - 2, 0, "-" * w, w)
    safe_addstr(stdscr, h - 1, 2, text, w - 4, curses.A_DIM)


def render_progress_bar(pct: float, bar_w: int, style: str = "blocks") -> str:
    pct = max(0.0, min(100.0, pct))
    inner_w = max(1, bar_w - 2)
    filled = int(inner_w * (pct / 100.0))
    filled = max(0, min(inner_w, filled))
    unfilled = inner_w - filled

    if style == "blocks":
        bar = "█" * filled + "░" * unfilled
    elif style == "dots":
        bar = "●" * filled + "○" * unfilled
    elif style == "minimal":
        bar = "#" * filled + "-" * unfilled
    else:  # "classic"
        arrow = ">" if filled < inner_w else ""
        spaces = " " * max(0, unfilled - len(arrow))
        bar = "=" * filled + arrow + spaces

    return f"[{bar}]"


def run_menu_tabbed(stdscr, title: str, tabs: list[dict], footer: str,
                       start_tab: int = 0, start_index: int = 0,
                       ascii_art: list[str] = None) -> tuple[int | None, int | None]:
    tab_idx = start_tab
    item_idx = start_index
    curses.curs_set(0)

    while True:
        stdscr.erase()
        h, w = stdscr.getmaxyx()
        draw_header(stdscr, title, w)
        row = 2

        # Render ASCII Art if provided and screen height is sufficient
        if ascii_art and h >= 24 and w >= 82:
            art_x = max(2, (w - max(len(line) for line in ascii_art)) // 2)
            for line in ascii_art:
                safe_addstr(stdscr, row, art_x, line, w - 4, curses.A_BOLD)
                row += 1
            row += 1

        # Render Tab Bar
        tab_x = 4
        for t_i, tab in enumerate(tabs):
            t_name = f" [ {tab['title']} ] "
            if t_i == tab_idx:
                safe_addstr(stdscr, row, tab_x, t_name, w - tab_x, HL_ATTR)
            else:
                safe_addstr(stdscr, row, tab_x, t_name, w - tab_x, curses.A_DIM)
            tab_x += len(t_name) + 2
        row += 2

        # Current Active Tab
        current_tab = tabs[tab_idx]
        items = current_tab["items"]
        descriptions = current_tab.get("descriptions", [])
        item_idx = min(item_idx, len(items) - 1) if items else 0

        # Description block reservation
        desc_text = descriptions[item_idx] if descriptions and 0 <= item_idx < len(descriptions) else ""
        desc_lines = desc_text.split("\n") if desc_text else []
        reserved_bottom_rows = 3 + (len(desc_lines) + 2 if desc_lines else 0)

        for i, item in enumerate(items):
            y = row + i
            if y >= h - reserved_bottom_rows:
                break
            is_divider = item.startswith("---") or item.startswith("===")
            if is_divider:
                safe_addstr(stdscr, y, 4, item, w - 8, curses.A_DIM)
                continue

            attr = HL_ATTR if i == item_idx else curses.A_NORMAL
            marker = "> " if i == item_idx else "  "
            safe_addstr(stdscr, y, 4, f"{marker}{item}", w - 8, attr)

        # Render Description box at bottom
        if desc_lines:
            desc_y = h - 2 - len(desc_lines) - 2
            safe_addstr(stdscr, desc_y, 2, "-" * (w - 4), w - 4, curses.A_DIM)
            safe_addstr(stdscr, desc_y + 1, 4, "Info:", w - 8, curses.A_BOLD)
            for dl_idx, dl in enumerate(desc_lines):
                safe_addstr(stdscr, desc_y + 2 + dl_idx, 4, dl, w - 8, curses.A_DIM)

        draw_footer(stdscr, footer, w, h)
        stdscr.refresh()

        key = stdscr.getch()

        if key == 17:  # Ctrl+Q
            return None, None
        if key == 3:   # Ctrl+C
            continue

        if key in (curses.KEY_LEFT, ord('h')):
            tab_idx = (tab_idx - 1) % len(tabs)
            item_idx = 0
        elif key in (curses.KEY_RIGHT, ord('l')):
            tab_idx = (tab_idx + 1) % len(tabs)
            item_idx = 0
        elif key in (curses.KEY_UP, ord('k')):
            if items:
                item_idx = (item_idx - 1) % len(items)
                while items[item_idx].startswith("---") or items[item_idx].startswith("==="):
                    item_idx = (item_idx - 1) % len(items)
        elif key in (curses.KEY_DOWN, ord('j')):
            if items:
                item_idx = (item_idx + 1) % len(items)
                while items[item_idx].startswith("---") or items[item_idx].startswith("==="):
                    item_idx = (item_idx + 1) % len(items)
        elif key in (curses.KEY_ENTER, 10, 13):
            return tab_idx, item_idx
        elif key in (27, ord('q')):
            return None, None


def run_menu(stdscr, title: str, items: list[str], footer: str, subtitle: str = "",
             start_index: int = 0, descriptions: list[str] = None) -> int | None:
    """Standard single-list menu widget."""
    idx = min(start_index, len(items) - 1) if items else 0
    curses.curs_set(0)
    while True:
        stdscr.erase()
        h, w = stdscr.getmaxyx()
        draw_header(stdscr, title, w)
        row = 2

        if subtitle:
            sub_lines = subtitle.split("\n")
            for j, sline in enumerate(sub_lines):
                safe_addstr(stdscr, row + j, 2, sline, w - 4, curses.A_DIM)
            row += len(sub_lines) + 1

        desc_text = descriptions[idx] if descriptions and 0 <= idx < len(descriptions) else ""
        desc_lines = desc_text.split("\n") if desc_text else []
        reserved_bottom_rows = 3 + (len(desc_lines) + 2 if desc_lines else 0)

        for i, item in enumerate(items):
            y = row + i
            if y >= h - reserved_bottom_rows:
                break
            if item.startswith("---") or item.startswith("==="):
                safe_addstr(stdscr, y, 4, item, w - 8, curses.A_DIM)
                continue

            attr = HL_ATTR if i == idx else curses.A_NORMAL
            marker = "> " if i == idx else "  "
            safe_addstr(stdscr, y, 4, f"{marker}{item}", w - 8, attr)

        if desc_lines:
            desc_y = h - 2 - len(desc_lines) - 2
            safe_addstr(stdscr, desc_y, 2, "-" * (w - 4), w - 4, curses.A_DIM)
            safe_addstr(stdscr, desc_y + 1, 4, "Info / Description:", w - 8, curses.A_BOLD)
            for dl_idx, dl in enumerate(desc_lines):
                safe_addstr(stdscr, desc_y + 2 + dl_idx, 4, dl, w - 8, curses.A_DIM)

        draw_footer(stdscr, footer, w, h)
        stdscr.refresh()

        key = stdscr.getch()

        if key == 17:  # Ctrl+Q
            return None
        if key == 3:   # Ctrl+C
            continue

        if not items:
            if key in (27, ord('q')):
                return None
            continue
        if key in (curses.KEY_UP, ord('k')):
            idx = (idx - 1) % len(items)
            while items[idx].startswith("---") or items[idx].startswith("==="):
                idx = (idx - 1) % len(items)
        elif key in (curses.KEY_DOWN, ord('j')):
            idx = (idx + 1) % len(items)
            while items[idx].startswith("---") or items[idx].startswith("==="):
                idx = (idx + 1) % len(items)
        elif key in (curses.KEY_ENTER, 10, 13):
            return idx
        elif key in (27, ord('q')):
            return None


def text_input(stdscr, title: str, prompt: str, footer: str, default: str = "") -> str | None:
    buf = list(default)
    curses.curs_set(1)
    while True:
        stdscr.erase()
        h, w = stdscr.getmaxyx()
        draw_header(stdscr, title, w)
        safe_addstr(stdscr, 3, 4, prompt, w - 8)
        input_str = "".join(buf)
        safe_addstr(stdscr, 5, 4, "> " + input_str, w - 8)
        draw_footer(stdscr, footer, w, h)
        cursor_x = min(6 + len(buf), w - 1)
        try:
            stdscr.move(5, cursor_x)
        except curses.error:
            pass
        stdscr.refresh()

        key = stdscr.getch()

        if key == 17 or key == 27:
            curses.curs_set(0)
            return None
        if key == 3:
            continue

        if key in (curses.KEY_ENTER, 10, 13):
            curses.curs_set(0)
            return "".join(buf)
        if key in (curses.KEY_BACKSPACE, 127, 8):
            if buf:
                buf.pop()
        elif key == curses.KEY_RESIZE:
            continue
        elif 32 <= key <= 126:
            buf.append(chr(key))


def show_message(stdscr, title: str, lines: list[str], footer: str) -> None:
    curses.curs_set(0)
    while True:
        stdscr.erase()
        h, w = stdscr.getmaxyx()
        draw_header(stdscr, title, w)
        for i, line in enumerate(lines):
            y = 3 + i
            if y >= h - 2:
                break
            safe_addstr(stdscr, y, 4, line, w - 8)
        draw_footer(stdscr, footer, w, h)
        stdscr.refresh()
        key = stdscr.getch()
        if key in (curses.KEY_ENTER, 10, 13, 27, ord('q'), 17):
            return


def run_external(stdscr, cmd: list[str]) -> None:
    """Suspends curses and runs cmd on the raw terminal."""
    curses.def_prog_mode()
    curses.endwin()
    print("\n[cmd]", " ".join(cmd), "\n")
    try:
        subprocess.run(cmd, check=False)
    except FileNotFoundError:
        print("[!] Command not found.")
    except KeyboardInterrupt:
        print("\n[!] Interrupted.")
    input("\nPress Enter to return to the menu...")
    curses.reset_prog_mode()
    stdscr.clear()
    curses.curs_set(0)


def parse_sec(h: str, m: str, s: str) -> float:
    return int(h) * 3600 + int(m) * 60 + float(s)


def run_with_log(stdscr, cfg: dict, cmd: list[str], op_type: str = "Task",
                 source: str = "", target: str = "") -> None:
    """Multi-stage process executor with dynamic active stages and live progress indicators."""
    curses.curs_set(0)
    lang = cfg.get("language", "en")
    style = cfg.get("progress_style", "blocks")

    # Inject Proxy if configured
    proxy_val = cfg.get("proxy", "").strip()
    if proxy_val and "yt-dlp" in cmd[0] and "--proxy" not in cmd:
        cmd += ["--proxy", proxy_val]

    try:
        proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=0)
    except FileNotFoundError:
        show_message(stdscr, t(cfg, "log_title"), [t(cfg, "info_not_installed")], t(cfg, "footer_message"))
        return

    # Build Dynamic Stages List (ONLY include stages active in command)
    stages = []
    if "yt-dlp" in cmd[0]:
        stages.append({"id": "meta", "name_en": "1. Extracting URL & Media Metadata", "name_ru": "1. Анализ ссылки и метаданных", "status": "active"})
        if "--skip-download" not in cmd:
            stages.append({"id": "video", "name_en": "2. Downloading Video Stream", "name_ru": "2. Загрузка видеопотока", "status": "pending"})
            if "--audio-multistreams" in cmd or "mergeall" in " ".join(cmd):
                stages.append({"id": "audio", "name_en": "3. Downloading Audio Stream(s)", "name_ru": "3. Загрузка аудиопотоков", "status": "pending"})
            if "--recode-video" in cmd or "--postprocessor-args" in cmd or "--merge-output-format" in cmd:
                stages.append({"id": "merge", "name_en": "4. Stream Merging & Codec Processing", "name_ru": "4. Обработка кодеков и сведение FFmpeg", "status": "pending"})
            if "--embed-metadata" in cmd or "--embed-chapters" in cmd or "--embed-subs" in cmd:
                stages.append({"id": "embed", "name_en": "5. Embedding Chapters, Metadata & Subs", "name_ru": "5. Вшивание глав, метаданных и субтитров", "status": "pending"})
    else:  # FFmpeg
        stages.append({"id": "init", "name_en": "1. Analyzing Input File & Streams", "name_ru": "1. Анализ исходного файла и потоков", "status": "active"})
        stages.append({"id": "transcode", "name_en": "2. Transcoding Video & Audio (FFmpeg)", "name_ru": "2. Перекодирование медиапотоков (FFmpeg)", "status": "pending"})
        stages.append({"id": "finish", "name_en": "3. Finalizing Container & Output File", "name_ru": "3. Завершение и запись файла", "status": "pending"})

    stdscr.nodelay(True)
    lines: list[str] = [f"[cmd] {' '.join(cmd)}", ""]
    current = ""
    buf = ""
    eof = False
    show_logs = False

    # Progress tracking states
    pct = 0.0
    total_sec = 0.0
    curr_sec = 0.0
    speed_str = ""
    fps_str = ""
    frame_str = ""
    size_str = ""
    spinner_frames = ["|", "/", "-", "\\"]
    spinner_idx = 0

    re_pct = re.compile(r'(\d+(?:\.\d+)?)%')
    re_dur = re.compile(r'Duration:\s*(\d+):(\d+):(\d+(?:\.\d+)?)')
    re_time = re.compile(r'time=(\d+):(\d+):(\d+(?:\.\d+)?)')
    re_speed = re.compile(r'speed=\s*(\S+)')
    re_fps = re.compile(r'fps=\s*(\d+)')
    re_frame = re.compile(r'frame=\s*(\d+)')
    re_size = re.compile(r'(?:Lsize|size)=\s*(\S+)')

    def update_stage_and_progress(text: str):
        nonlocal pct, total_sec, curr_sec, speed_str, fps_str, frame_str, size_str

        # Stage Transition Parsing
        if "yt-dlp" in cmd[0]:
            if "[download] Destination:" in text or "[download]" in text:
                for st in stages:
                    if st["id"] == "meta":
                        st["status"] = "done"
                    elif st["id"] == "video":
                        st["status"] = "active"
            elif "[Merger]" in text or "[ffmpeg]" in text or "[VideoConvertor]" in text or "Converting" in text:
                for st in stages:
                    if st["id"] in ("meta", "video", "audio"):
                        st["status"] = "done"
                    elif st["id"] == "merge":
                        st["status"] = "active"
            elif "[Metadata]" in text or "[EmbedSubtitle]" in text or "[Thumb]" in text or "[Exec]" in text:
                for st in stages:
                    if st["id"] in ("meta", "video", "audio", "merge"):
                        st["status"] = "done"
                    elif st["id"] == "embed":
                        st["status"] = "active"
        else:  # FFmpeg
            if "frame=" in text or "time=" in text:
                stages[0]["status"] = "done"
                if len(stages) > 1:
                    stages[1]["status"] = "active"
            elif "[out#0" in text or "video:" in text:
                for st in stages[:-1]:
                    st["status"] = "done"
                stages[-1]["status"] = "active"

        # Regex Parsing
        m_dur = re_dur.search(text)
        if m_dur:
            total_sec = parse_sec(*m_dur.groups())

        m_time = re_time.search(text)
        if m_time:
            curr_sec = parse_sec(*m_time.groups())
            if total_sec > 0:
                pct = min(100.0, (curr_sec / total_sec) * 100.0)

        m_pct = re_pct.search(text)
        if m_pct:
            try:
                pct = float(m_pct.group(1))
            except ValueError:
                pass

        m_speed = re_speed.search(text)
        if m_speed:
            speed_str = m_speed.group(1)

        m_fps = re_fps.search(text)
        if m_fps:
            fps_str = m_fps.group(1)

        m_frame = re_frame.search(text)
        if m_frame:
            frame_str = m_frame.group(1)

        m_size = re_size.search(text)
        if m_size:
            size_str = m_size.group(1)

    f10_key = getattr(curses, "KEY_F10", 274)

    while not eof:
        spinner_idx = (spinner_idx + 1) % len(spinner_frames)
        try:
            key = stdscr.getch()
        except curses.error:
            key = -1

        if key in (f10_key, 274):
            show_logs = not show_logs

        if key in (ord('q'), 27, 17) and proc.poll() is None:
            proc.terminate()

        readable, _, _ = select.select([proc.stdout], [], [], 0.05)
        if readable:
            chunk = proc.stdout.read(4096)
            if chunk == b"":
                eof = True
            else:
                text_chunk = chunk.decode("utf-8", errors="replace")
                buf += text_chunk
                update_stage_and_progress(text_chunk)
                while True:
                    idx_r = buf.find("\r")
                    idx_n = buf.find("\n")
                    if idx_r == -1 and idx_n == -1:
                        current = buf
                        buf = ""
                        break
                    if idx_n != -1 and (idx_r == -1 or idx_n < idx_r):
                        piece = buf[:idx_n]
                        if piece:
                            lines.append(piece)
                            update_stage_and_progress(piece)
                        buf = buf[idx_n + 1:]
                        current = ""
                    else:
                        piece = buf[:idx_r]
                        if piece:
                            update_stage_and_progress(piece)
                        current = piece
                        buf = buf[idx_r + 1:]

        h, w = stdscr.getmaxyx()
        stdscr.erase()
        draw_header(stdscr, t(cfg, "log_title"), w)

        if show_logs:
            log_h = max(0, h - 4)
            display = (lines + ([current] if current else []))[-log_h:] if log_h else []
            for i, line in enumerate(display):
                safe_addstr(stdscr, 3 + i, 2, line, w - 4)
        else:
            y = 3
            safe_addstr(stdscr, y, 4, f"Operation: {op_type}", w - 8, curses.A_BOLD)
            if source:
                safe_addstr(stdscr, y + 1, 4, f"Source: {source}", w - 8, curses.A_DIM)
            y += 3

            safe_addstr(stdscr, y, 4, "Execution Stages:", w - 8, curses.A_BOLD)
            y += 1

            for st in stages:
                st_name = st["name_ru"] if lang == "ru" else st["name_en"]
                status = st["status"]

                if status == "done":
                    icon = "[✓]"
                    attr = curses.A_DIM
                elif status == "active":
                    icon = f"[{spinner_frames[spinner_idx]}]"
                    attr = curses.A_BOLD
                else:
                    icon = "[ ]"
                    attr = curses.A_DIM

                safe_addstr(stdscr, y, 6, f"{icon} {st_name}", w - 10, attr)
                y += 1

                if status == "active":
                    bar_str = render_progress_bar(pct, min(40, max(20, w - 30)), style)
                    prog_str = f"     {bar_str} {pct:5.1f}%"
                    if speed_str or fps_str:
                        prog_str += f" ({speed_str or fps_str or ''})"
                    safe_addstr(stdscr, y, 6, prog_str, w - 10, curses.A_BOLD)
                    y += 1

            y += 1
            safe_addstr(stdscr, y, 4, "[Press F10 to show/hide raw terminal log]", w - 8, curses.A_DIM)

        draw_footer(stdscr, t(cfg, "log_footer_running"), w, h)
        stdscr.refresh()

    proc.wait()
    stdscr.nodelay(False)
    rc = proc.returncode

    if rc == 0:
        for st in stages:
            st["status"] = "done"

    status = "Success" if rc == 0 else f"Failed ({rc})"
    status_msg = t(cfg, "log_finished_ok") if rc == 0 else t(cfg, "log_finished_err", v=rc)
    lines += ["", status_msg]

    if source or target:
        add_history_entry(op_type, source, target, status)

    h, w = stdscr.getmaxyx()
    stdscr.erase()
    draw_header(stdscr, t(cfg, "log_title"), w)
    log_h = max(0, h - 4)
    for i, line in enumerate(lines[-log_h:]):
        safe_addstr(stdscr, 3 + i, 2, line, w - 4)
    draw_footer(stdscr, t(cfg, "log_footer_done"), w, h)
    stdscr.refresh()
    curses.curs_set(0)
    while True:
        k = stdscr.getch()
        if k in (curses.KEY_ENTER, 10, 13, 27, ord('q'), 17):
            break
    stdscr.clear()


# ==========================================================================
# First-Run Wizard
# ==========================================================================

def run_first_run_wizard(stdscr, cfg: dict) -> dict:
    """Step-by-step onboarding wizard for first-time users."""
    apply_theme(cfg)

    # Step 1: Language
    lang_idx = run_menu(stdscr, t(cfg, "wizard_title"), ["English", "Русский"], t(cfg, "footer_nav"),
                        subtitle=t(cfg, "wizard_step1_title"))
    if lang_idx is not None:
        cfg["language"] = "en" if lang_idx == 0 else "ru"

    # Step 2: Usage Goal / Purpose
    goals = ["editing", "downloading", "audio", "transcoding"]
    goal_labels = [
        t(cfg, "goal_editing"),
        t(cfg, "goal_downloading"),
        t(cfg, "goal_audio"),
        t(cfg, "goal_transcoding"),
    ]
    g_idx = run_menu(stdscr, t(cfg, "wizard_title"), goal_labels, t(cfg, "footer_nav"),
                     subtitle=f"{t(cfg, 'wizard_step2_title')}\n{t(cfg, 'wizard_step2_subtitle')}")
    if g_idx is not None:
        cfg["user_goal"] = goals[g_idx]
        if cfg["user_goal"] == "editing":
            cfg["video_preset"] = "davinci_dnxhr"
        elif cfg["user_goal"] in ("downloading", "transcoding"):
            cfg["video_preset"] = "standard_mp4"

    # Step 3: Default download dir
    out = text_input(stdscr, t(cfg, "wizard_title"), t(cfg, "settings_new_download_dir"),
                     t(cfg, "footer_input"), default=cfg["download_dir"])
    if out:
        cfg["download_dir"] = out

    # Step 4: Color Theme
    lang = cfg.get("language", "en")
    theme_keys = list(THEMES.keys())
    theme_labels = [THEMES[k]["name_ru"] if lang == "ru" else THEMES[k]["name_en"] for k in theme_keys]
    th_idx = run_menu(stdscr, t(cfg, "wizard_title"), theme_labels, t(cfg, "footer_nav"),
                      subtitle=t(cfg, "wizard_step4_title"))
    if th_idx is not None:
        cfg["theme"] = theme_keys[th_idx]

    # Step 4b: Keep terminal background toggle
    bg_idx = run_menu(stdscr, t(cfg, "wizard_title"),
                      [t(cfg, "bg_option_keep"), t(cfg, "bg_option_solid")], t(cfg, "footer_nav"),
                      subtitle=t(cfg, "bg_option_title"))
    if bg_idx is not None:
        cfg["use_terminal_bg"] = (bg_idx == 0)

    apply_theme(cfg)
    cfg["first_run_completed"] = True
    save_config(cfg)

    show_message(stdscr, t(cfg, "wizard_done_title"), [t(cfg, "wizard_done_msg")], t(cfg, "footer_message"))
    return cfg


# ==========================================================================
# Action screens
# ==========================================================================

def select_preset_menu(stdscr, cfg: dict) -> tuple[str | None, list[str]]:
    lang = cfg.get("language", "en")
    preset_keys = get_ordered_video_preset_keys(cfg)
    items = [VIDEO_PRESETS[pk]["name_ru"] if lang == "ru" else VIDEO_PRESETS[pk]["name_en"] for pk in preset_keys]
    descs = [VIDEO_PRESETS[pk]["desc_ru"] if lang == "ru" else VIDEO_PRESETS[pk]["desc_en"] for pk in preset_keys]

    curr_preset = cfg.get("video_preset", "davinci_dnxhr")
    default_idx = preset_keys.index(curr_preset) if curr_preset in preset_keys else 0

    pi = run_menu(stdscr, t(cfg, "video_title"), items, t(cfg, "footer_nav"),
                  subtitle=t(cfg, "preset_subtitle"), start_index=default_idx, descriptions=descs)
    if pi is None:
        return None, []

    selected_key = preset_keys[pi]
    if selected_key == "custom":
        ext = text_input(stdscr, t(cfg, "video_title"), t(cfg, "custom_ext_prompt"), t(cfg, "footer_input"), default="mp4")
        if not ext:
            return None, []
        flags = text_input(stdscr, t(cfg, "video_title"), t(cfg, "custom_flags_prompt"), t(cfg, "footer_input"), default="-c:v libx264 -c:a aac")
        if flags is None:
            return None, []
        return "custom", ["--recode-video", ext.strip("."), "--postprocessor-args", f"ffmpeg:{flags}"]

    return selected_key, VIDEO_PRESETS[selected_key]["args"]


def configure_advanced_video_options(stdscr, cfg: dict) -> dict | None:
    """Sub-menu for configuring advanced YouTube download flags with grouped dividers and Proceed at top."""
    adv = {
        "audio_track": "default",      # "default" | "all" | "ru" | "en" | "custom"
        "custom_audio_lang": "",
        "vcodec": "auto",              # "auto" | "av1" | "vp9" | "h264"
        "sponsorblock": "off",         # "off" | "sponsors" | "sponsors_promo"
        "geobypass": False,
        "ratelimit": "",               # "" | "10M" | "5M" | "2M"
        "clip_range": "",              # "" or "*00:01:00-00:02:30"
        "live_start": False,
        "write_extra": False,
        "embed_metadata": True,
        "embed_chapters": True,
        "embed_subs": False,
        "fps_limit": "",               # "" | "30" | "60"
        "proxy": cfg.get("proxy", ""),
    }

    while True:
        # Format Labels
        if adv["audio_track"] == "all":
            aud_str = t(cfg, "adv_audio_all")
        elif adv["audio_track"] == "ru":
            aud_str = t(cfg, "adv_audio_ru")
        elif adv["audio_track"] == "en":
            aud_str = t(cfg, "adv_audio_en")
        elif adv["audio_track"] == "custom":
            aud_str = f"{t(cfg, 'adv_audio_custom')} ({adv['custom_audio_lang']})"
        else:
            aud_str = t(cfg, "adv_audio_default")

        vcodec_map = {"auto": t(cfg, "adv_vcodec_auto"), "av1": t(cfg, "adv_vcodec_av1"),
                      "vp9": t(cfg, "adv_vcodec_vp9"), "h264": t(cfg, "adv_vcodec_h264")}
        vcodec_str = vcodec_map.get(adv["vcodec"], t(cfg, "adv_vcodec_auto"))

        sb_map = {"off": t(cfg, "adv_sb_off"), "sponsors": t(cfg, "adv_sb_sponsors"), "sponsors_promo": t(cfg, "adv_sb_all")}
        sb_str = sb_map.get(adv["sponsorblock"], t(cfg, "adv_sb_off"))

        clip_str = adv["clip_range"] if adv["clip_range"] else t(cfg, "adv_clip_full")
        fps_str = adv["fps_limit"] + " FPS" if adv["fps_limit"] else t(cfg, "adv_fps_max")
        rate_str = adv["ratelimit"] if adv["ratelimit"] else t(cfg, "adv_ratelimit_max")
        proxy_str = adv["proxy"] if adv["proxy"] else t(cfg, "adv_proxy_none")

        # Items list with Proceed at TOP and dividers
        items = [
            t(cfg, "adv_proceed"),
            "--- Audio & Video Codecs ---",
            t(cfg, "adv_audio_track", v=aud_str),
            t(cfg, "adv_vcodec", v=vcodec_str),
            t(cfg, "adv_fps", v=fps_str),
            "--- SponsorBlock & Clipping ---",
            t(cfg, "adv_sponsorblock", v=sb_str),
            t(cfg, "adv_clip", v=clip_str),
            t(cfg, "adv_ratelimit", v=rate_str),
            "--- Metadata & Embeds ---",
            t(cfg, "adv_embed_meta", v=("YES" if adv["embed_metadata"] else "NO")),
            t(cfg, "adv_embed_chap", v=("YES" if adv["embed_chapters"] else "NO")),
            t(cfg, "adv_embed_subs", v=("YES" if adv["embed_subs"] else "NO")),
            t(cfg, "adv_write_extra", v=("YES" if adv["write_extra"] else "NO")),
            "--- Network & Unblock ---",
            t(cfg, "adv_geobypass", v=("YES" if adv["geobypass"] else "NO")),
            t(cfg, "adv_proxy", v=proxy_str),
        ]

        choice = run_menu(stdscr, t(cfg, "adv_title"), items, t(cfg, "footer_nav"))
        if choice is None:
            return None
        if choice == 0:  # Proceed is at TOP!
            return adv

        if choice == 2:  # Audio Tracks
            a_opts = [t(cfg, "adv_audio_default"), t(cfg, "adv_audio_all"), t(cfg, "adv_audio_ru"), t(cfg, "adv_audio_en"), t(cfg, "adv_audio_custom")]
            ai = run_menu(stdscr, t(cfg, "adv_title"), a_opts, t(cfg, "footer_nav"))
            if ai is not None:
                keys = ["default", "all", "ru", "en", "custom"]
                adv["audio_track"] = keys[ai]
                if keys[ai] == "custom":
                    clang = text_input(stdscr, t(cfg, "adv_title"), t(cfg, "adv_audio_prompt"), t(cfg, "footer_input"))
                    if clang:
                        adv["custom_audio_lang"] = clang.strip()

        elif choice == 3:  # Video Codec
            vc_opts = [t(cfg, "adv_vcodec_auto"), t(cfg, "adv_vcodec_av1"), t(cfg, "adv_vcodec_vp9"), t(cfg, "adv_vcodec_h264")]
            vci = run_menu(stdscr, t(cfg, "adv_title"), vc_opts, t(cfg, "footer_nav"))
            if vci is not None:
                adv["vcodec"] = ["auto", "av1", "vp9", "h264"][vci]

        elif choice == 4:  # FPS Limit
            fps_opts = [t(cfg, "adv_fps_max"), "60 FPS", "30 FPS"]
            fi = run_menu(stdscr, t(cfg, "adv_title"), fps_opts, t(cfg, "footer_nav"))
            if fi is not None:
                adv["fps_limit"] = ["", "60", "30"][fi]

        elif choice == 6:  # SponsorBlock
            sb_opts = [t(cfg, "adv_sb_off"), t(cfg, "adv_sb_sponsors"), t(cfg, "adv_sb_all")]
            sbi = run_menu(stdscr, t(cfg, "adv_title"), sb_opts, t(cfg, "footer_nav"))
            if sbi is not None:
                adv["sponsorblock"] = ["off", "sponsors", "sponsors_promo"][sbi]

        elif choice == 7:  # Time clip
            c_val = text_input(stdscr, t(cfg, "adv_title"), t(cfg, "adv_clip_prompt"), t(cfg, "footer_input"), default=adv["clip_range"])
            if c_val is not None:
                adv["clip_range"] = c_val.strip()

        elif choice == 8:  # Rate limit
            rl_opts = [t(cfg, "adv_ratelimit_max"), "10M", "5M", "2M"]
            rli = run_menu(stdscr, t(cfg, "adv_title"), rl_opts, t(cfg, "footer_nav"))
            if rli is not None:
                adv["ratelimit"] = ["", "10M", "5M", "2M"][rli]

        elif choice == 10:  # Embed Metadata
            adv["embed_metadata"] = not adv["embed_metadata"]
        elif choice == 11:  # Embed Chapters
            adv["embed_chapters"] = not adv["embed_chapters"]
        elif choice == 12:  # Embed Subtitles
            adv["embed_subs"] = not adv["embed_subs"]
        elif choice == 13:  # Write extra files
            adv["write_extra"] = not adv["write_extra"]

        elif choice == 15:  # Geo-bypass
            adv["geobypass"] = not adv["geobypass"]
        elif choice == 16:  # Proxy
            pr = text_input(stdscr, t(cfg, "adv_title"), t(cfg, "adv_proxy_prompt"), t(cfg, "footer_input"), default=adv["proxy"])
            if pr is not None:
                adv["proxy"] = pr.strip()


def screen_video(stdscr, cfg: dict) -> None:
    url = text_input(stdscr, t(cfg, "video_title"), t(cfg, "video_prompt_url"), t(cfg, "footer_input"))
    if not url:
        return

    quality_opts = [(t(cfg, "quality_best"), ""), ("2160p (4K)", "2160"), ("1440p (2K)", "1440"),
                     ("1080p", "1080"), ("720p", "720"), ("480p", "480"), (t(cfg, "quality_custom"), "custom")]
    labels = [label for label, _ in quality_opts]
    qi = run_menu(stdscr, t(cfg, "video_title"), labels, t(cfg, "footer_nav"),
                  subtitle=t(cfg, "video_quality_subtitle"))
    if qi is None:
        return
    quality = quality_opts[qi][1]
    if quality == "custom":
        quality = text_input(stdscr, t(cfg, "video_title"), t(cfg, "quality_custom_prompt"),
                              t(cfg, "footer_input")) or ""

    preset_id, preset_args = select_preset_menu(stdscr, cfg)
    if preset_id is None:
        return

    subs_choice = run_menu(stdscr, t(cfg, "video_title"),
                            [t(cfg, "subs_choice_none"), t(cfg, "subs_choice_yes")], t(cfg, "footer_nav"))
    if subs_choice is None:
        return
    subs = None
    if subs_choice == 1:
        subs = text_input(stdscr, t(cfg, "video_title"), t(cfg, "subs_langs_prompt"),
                           t(cfg, "footer_input"), default=cfg["sub_langs"])

    # Configure Advanced Options
    adv = configure_advanced_video_options(stdscr, cfg)
    if adv is None:
        return

    out_dir = text_input(stdscr, t(cfg, "video_title"), t(cfg, "outdir_prompt"),
                          t(cfg, "footer_input"), default=cfg["download_dir"])
    if out_dir is None:
        return

    expanded_out = Path(out_dir).expanduser()
    expanded_out.mkdir(parents=True, exist_ok=True)

    # Build Advanced yt-dlp command
    cmd = ["yt-dlp"]

    fps_suffix = f"[fps<={adv['fps_limit']}]" if adv['fps_limit'] else ""
    q_str = f"[height<={quality}]" if quality else ""

    vc_filter = ""
    if adv["vcodec"] == "av1":
        vc_filter = "[vcodec^=av01]"
    elif adv["vcodec"] == "vp9":
        vc_filter = "[vcodec^=vp9]"
    elif adv["vcodec"] == "h264":
        vc_filter = "[vcodec^=avc1]"

    if adv["audio_track"] == "all":
        cmd += ["--audio-multistreams"]
        fmt = f"bestvideo{q_str}{fps_suffix}{vc_filter}+mergeall[format_id*=audio]/bestvideo{q_str}{fps_suffix}{vc_filter}+bestaudio/best"
    elif adv["audio_track"] == "ru":
        fmt = f"bestvideo{q_str}{fps_suffix}{vc_filter}+bestaudio[language=ru]/bestvideo{q_str}{fps_suffix}{vc_filter}+bestaudio[language^=ru]/bestvideo{q_str}{fps_suffix}{vc_filter}+bestaudio"
    elif adv["audio_track"] == "en":
        fmt = f"bestvideo{q_str}{fps_suffix}{vc_filter}+bestaudio[language=en]/bestvideo{q_str}{fps_suffix}{vc_filter}+bestaudio[language^=en]/bestvideo{q_str}{fps_suffix}{vc_filter}+bestaudio"
    elif adv["audio_track"] == "custom" and adv["custom_audio_lang"]:
        clang = adv["custom_audio_lang"].strip()
        fmt = f"bestvideo{q_str}{fps_suffix}{vc_filter}+bestaudio[language={clang}]/bestvideo{q_str}{fps_suffix}{vc_filter}+bestaudio"
    else:
        fmt = f"bestvideo{q_str}{fps_suffix}{vc_filter}+bestaudio/best{q_str}{fps_suffix}"

    cmd += ["-f", fmt]

    if adv["geobypass"]:
        cmd += ["--geo-bypass"]
    if adv["proxy"]:
        cmd += ["--proxy", adv["proxy"]]
    if adv["ratelimit"]:
        cmd += ["--limit-rate", adv["ratelimit"]]
    if adv["live_start"]:
        cmd += ["--live-from-start"]
    if adv["write_extra"]:
        cmd += ["--write-description", "--write-thumbnail"]

    if adv["sponsorblock"] == "sponsors":
        cmd += ["--sponsorblock-remove", "sponsor"]
    elif adv["sponsorblock"] == "sponsors_promo":
        cmd += ["--sponsorblock-remove", "sponsor,selfpromo,interaction"]

    if adv["clip_range"]:
        clip_val = adv["clip_range"].strip()
        if not clip_val.startswith("*"):
            clip_val = f"*{clip_val}"
        cmd += ["--download-sections", clip_val]

    if adv["embed_metadata"]:
        cmd += ["--embed-metadata", "--embed-thumbnail"]
    if adv["embed_chapters"]:
        cmd += ["--embed-chapters"]
    if adv["embed_subs"] and subs:
        cmd += ["--embed-subs"]

    if subs:
        cmd += ["--write-subs", "--sub-langs", subs]

    cmd += ["-o", str(expanded_out / "%(title)s.%(ext)s"), *preset_args, *cookie_args(cfg), url]

    q_display = quality or t(cfg, "quality_best_short")
    p_display = get_preset_name(cfg, preset_id) if preset_id != "custom" else "Custom FFmpeg"
    cmd_str = " ".join(cmd)
    subtitle = (f"{t(cfg, 'label_url')} {url}\n"
                f"{t(cfg, 'label_quality')} {q_display}   {t(cfg, 'label_preset')} {p_display}\n"
                f"{t(cfg, 'label_folder')} {out_dir}\n"
                f"{t(cfg, 'label_cmd')} {cmd_str}")
    confirm = run_menu(stdscr, t(cfg, "confirm_title"), [t(cfg, "confirm_start"), t(cfg, "confirm_cancel")],
                        t(cfg, "footer_nav"), subtitle=subtitle)
    if confirm == 0:
        run_with_log(stdscr, cfg, cmd, op_type="Download Video", source=url, target=str(expanded_out))


def screen_convert(stdscr, cfg: dict) -> None:
    """Dedicated screen for converting local media files using FFmpeg."""
    file_path_str = text_input(stdscr, t(cfg, "convert_title"), t(cfg, "convert_prompt_file"), t(cfg, "footer_input"))
    if not file_path_str:
        return

    inp_path = Path(file_path_str.strip()).expanduser()
    if not inp_path.exists() or not inp_path.is_file():
        show_message(stdscr, t(cfg, "convert_title"), [t(cfg, "convert_err_notfound", f=str(inp_path))], t(cfg, "footer_message"))
        return

    lang = cfg.get("language", "en")
    presets_list = get_ordered_convert_presets(cfg)
    preset_items = [p["name_ru"] if lang == "ru" else p["name_en"] for p in presets_list]
    preset_descs = [p["desc_ru"] if lang == "ru" else p["desc_en"] for p in presets_list]

    pi = run_menu(stdscr, t(cfg, "convert_title"), preset_items, t(cfg, "footer_nav"),
                  subtitle=t(cfg, "convert_prompt_preset"), descriptions=preset_descs)
    if pi is None:
        return

    preset = presets_list[pi]

    if preset["id"] == "custom":
        ext = text_input(stdscr, t(cfg, "convert_title"), t(cfg, "custom_ext_prompt"), t(cfg, "footer_input"), default="mov")
        if not ext:
            return
        flags_str = text_input(stdscr, t(cfg, "convert_title"), t(cfg, "custom_flags_prompt"), t(cfg, "footer_input"), default="-c:v libx264 -c:a pcm_s16le")
        if flags_str is None:
            return
        ext = ext.strip(".")
        flags = flags_str.split()
        suffix = "_custom"
    else:
        ext = preset["ext"]
        flags = preset["ffmpeg_flags"]
        suffix = preset["suffix"]

    out_dir_str = text_input(stdscr, t(cfg, "convert_title"), t(cfg, "convert_prompt_outdir"),
                             t(cfg, "footer_input"), default=str(inp_path.parent))
    if out_dir_str is None:
        return

    out_dir = Path(out_dir_str.strip()).expanduser() if out_dir_str.strip() else inp_path.parent
    out_dir.mkdir(parents=True, exist_ok=True)

    out_file = out_dir / f"{inp_path.stem}{suffix}.{ext}"

    cmd = ["ffmpeg", "-y", "-i", str(inp_path), *flags, str(out_file)]

    preset_name = preset["name_ru"] if lang == "ru" else preset["name_en"]
    cmd_str = " ".join(cmd)
    subtitle = (f"{t(cfg, 'label_input')} {inp_path.name}\n"
                f"{t(cfg, 'label_output')} {out_file.name}\n"
                f"{t(cfg, 'label_preset')} {preset_name}\n"
                f"{t(cfg, 'label_cmd')} {cmd_str}")

    confirm = run_menu(stdscr, t(cfg, "confirm_title"), [t(cfg, "confirm_start"), t(cfg, "confirm_cancel")],
                        t(cfg, "footer_nav"), subtitle=subtitle)
    if confirm == 0:
        run_with_log(stdscr, cfg, cmd, op_type="Convert File", source=inp_path.name, target=out_file.name)


def screen_batch_convert(stdscr, cfg: dict) -> None:
    """Batch convert all media files in a selected directory."""
    folder_str = text_input(stdscr, t(cfg, "batch_title"), t(cfg, "batch_prompt_folder"), t(cfg, "footer_input"))
    if not folder_str:
        return

    folder_path = Path(folder_str.strip()).expanduser()
    if not folder_path.exists() or not folder_path.is_dir():
        show_message(stdscr, t(cfg, "batch_title"), [t(cfg, "convert_err_notfound", f=str(folder_path))], t(cfg, "footer_message"))
        return

    media_exts = {".mp4", ".mkv", ".mov", ".avi", ".webm", ".flv", ".m4v", ".ts", ".mp3", ".wav", ".flac"}
    files = [p for p in folder_path.iterdir() if p.is_file() and p.suffix.lower() in media_exts]

    if not files:
        show_message(stdscr, t(cfg, "batch_title"), [t(cfg, "batch_no_files")], t(cfg, "footer_message"))
        return

    lang = cfg.get("language", "en")
    presets_list = get_ordered_convert_presets(cfg)
    preset_items = [p["name_ru"] if lang == "ru" else p["name_en"] for p in presets_list]
    preset_descs = [p["desc_ru"] if lang == "ru" else p["desc_en"] for p in presets_list]

    pi = run_menu(stdscr, t(cfg, "batch_title"), preset_items, t(cfg, "footer_nav"),
                  subtitle=f"Found {len(files)} files. Select preset:", descriptions=preset_descs)
    if pi is None:
        return

    preset = presets_list[pi]
    out_dir = folder_path / f"converted_{preset['id']}"
    out_dir.mkdir(parents=True, exist_ok=True)

    for f in files:
        out_file = out_dir / f"{f.stem}{preset['suffix']}.{preset['ext']}"
        cmd = ["ffmpeg", "-y", "-i", str(f), *preset["ffmpeg_flags"], str(out_file)]
        run_with_log(stdscr, cfg, cmd, op_type=f"Batch ({f.name})", source=f.name, target=out_file.name)


def screen_trim(stdscr, cfg: dict) -> None:
    """Trim media file using FFmpeg start and end times."""
    file_path_str = text_input(stdscr, t(cfg, "trim_title"), t(cfg, "trim_prompt_file"), t(cfg, "footer_input"))
    if not file_path_str:
        return

    inp_path = Path(file_path_str.strip()).expanduser()
    if not inp_path.exists() or not inp_path.is_file():
        show_message(stdscr, t(cfg, "trim_title"), [t(cfg, "convert_err_notfound", f=str(inp_path))], t(cfg, "footer_message"))
        return

    start_time = text_input(stdscr, t(cfg, "trim_title"), t(cfg, "trim_prompt_start"), t(cfg, "footer_input"), default="00:00:00")
    if not start_time:
        return

    end_time = text_input(stdscr, t(cfg, "trim_title"), t(cfg, "trim_prompt_end"), t(cfg, "footer_input"))
    if end_time is None:
        return

    mode = run_menu(stdscr, t(cfg, "trim_title"), [t(cfg, "trim_mode_copy"), t(cfg, "trim_mode_reencode")], t(cfg, "footer_nav"))
    if mode is None:
        return

    out_file = inp_path.parent / f"{inp_path.stem}_trimmed{inp_path.suffix}"

    cmd = ["ffmpeg", "-y", "-ss", start_time.strip()]
    if end_time.strip():
        cmd += ["-to", end_time.strip()]
    cmd += ["-i", str(inp_path)]

    if mode == 0:
        cmd += ["-c", "copy", str(out_file)]
    else:
        preset = CONVERT_PRESETS[0]
        cmd += [*preset["ffmpeg_flags"], str(out_file)]

    confirm = run_menu(stdscr, t(cfg, "confirm_title"), [t(cfg, "confirm_start"), t(cfg, "confirm_cancel")],
                        t(cfg, "footer_nav"), subtitle=f"Command: {' '.join(cmd)}")
    if confirm == 0:
        run_with_log(stdscr, cfg, cmd, op_type="Trim File", source=inp_path.name, target=out_file.name)


def screen_probe(stdscr, cfg: dict) -> None:
    """Inspect local media file metadata using FFprobe."""
    file_path_str = text_input(stdscr, t(cfg, "probe_title"), t(cfg, "probe_prompt_file"), t(cfg, "footer_input"))
    if not file_path_str:
        return

    inp_path = Path(file_path_str.strip()).expanduser()
    if not inp_path.exists() or not inp_path.is_file():
        show_message(stdscr, t(cfg, "probe_title"), [t(cfg, "convert_err_notfound", f=str(inp_path))], t(cfg, "footer_message"))
        return

    cmd = ["ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", str(inp_path)]
    lines = []
    try:
        res = subprocess.run(cmd, capture_output=True, text=True, check=False)
        if res.returncode == 0:
            data = json.loads(res.stdout)
            fmt = data.get("format", {})
            lines.append(f"File: {inp_path.name}")
            lines.append(f"Size: {int(fmt.get('size', 0)) / (1024*1024):.2f} MB")
            lines.append(f"Duration: {float(fmt.get('duration', 0)):.2f} sec")
            lines.append(f"Bitrate: {int(fmt.get('bit_rate', 0)) // 1000} kbps")
            lines.append("")
            lines.append("Streams:")

            for i, st in enumerate(data.get("streams", [])):
                codec_type = st.get("codec_type", "unknown").upper()
                codec_name = st.get("codec_name", "unknown")
                if codec_type == "VIDEO":
                    w = st.get("width")
                    h = st.get("height")
                    fps = st.get("r_frame_rate", "?")
                    lines.append(f"  [{i}] {codec_type}: {codec_name} ({w}x{h}, {fps} fps)")
                elif codec_type == "AUDIO":
                    sr = st.get("sample_rate")
                    ch = st.get("channels")
                    lines.append(f"  [{i}] {codec_type}: {codec_name} ({sr} Hz, {ch} channels)")
                else:
                    lines.append(f"  [{i}] {codec_type}: {codec_name}")
        else:
            lines = ["FFprobe error:", res.stderr]
    except Exception as e:
        lines = [f"Failed to run FFprobe: {e}"]

    show_message(stdscr, t(cfg, "probe_title"), lines, t(cfg, "footer_message"))


def screen_history(stdscr, cfg: dict) -> None:
    """Display recent operations history."""
    if not HISTORY_PATH.exists():
        show_message(stdscr, t(cfg, "history_title"), [t(cfg, "history_empty")], t(cfg, "footer_message"))
        return

    try:
        entries = json.loads(HISTORY_PATH.read_text(encoding="utf-8"))
    except Exception:
        entries = []

    if not entries:
        show_message(stdscr, t(cfg, "history_title"), [t(cfg, "history_empty")], t(cfg, "footer_message"))
        return

    lines = []
    for item in entries[:25]:
        lines.append(f"[{item.get('time')}] {item.get('type')} ({item.get('status')})")
        lines.append(f"  Src: {item.get('source')}")
        lines.append(f"  Dst: {item.get('target')}")
        lines.append("")

    show_message(stdscr, t(cfg, "history_title"), lines, t(cfg, "footer_message"))


def screen_audio(stdscr, cfg: dict) -> None:
    url = text_input(stdscr, t(cfg, "audio_title"), t(cfg, "video_prompt_url"), t(cfg, "footer_input"))
    if not url:
        return

    fi = run_menu(stdscr, t(cfg, "audio_title"), AUDIO_FORMATS, t(cfg, "footer_nav"),
                  subtitle=t(cfg, "audio_format_subtitle"))
    if fi is None:
        return
    audio_format = AUDIO_FORMATS[fi]

    out_dir = text_input(stdscr, t(cfg, "audio_title"), t(cfg, "outdir_prompt"),
                          t(cfg, "footer_input"), default=cfg["download_dir"])
    if out_dir is None:
        return

    expanded_out = Path(out_dir).expanduser()
    expanded_out.mkdir(parents=True, exist_ok=True)
    cmd = ["yt-dlp", "-x", "--audio-format", audio_format, "--audio-quality", "0",
           "-o", str(expanded_out / "%(title)s.%(ext)s"), *cookie_args(cfg), url]

    cmd_str = " ".join(cmd)
    subtitle = (f"{t(cfg, 'label_url')} {url}\n"
                f"{t(cfg, 'label_format')} {audio_format}   {t(cfg, 'label_folder')} {out_dir}\n"
                f"{t(cfg, 'label_cmd')} {cmd_str}")
    confirm = run_menu(stdscr, t(cfg, "confirm_title"), [t(cfg, "confirm_start"), t(cfg, "confirm_cancel")],
                        t(cfg, "footer_nav"), subtitle=subtitle)
    if confirm == 0:
        run_with_log(stdscr, cfg, cmd, op_type="Download Audio", source=url, target=str(expanded_out))


def screen_playlist(stdscr, cfg: dict) -> None:
    url = text_input(stdscr, t(cfg, "playlist_title"), t(cfg, "playlist_prompt_url"), t(cfg, "footer_input"))
    if not url:
        return

    mode = run_menu(stdscr, t(cfg, "playlist_title"),
                     [t(cfg, "playlist_mode_video"), t(cfg, "playlist_mode_audio")], t(cfg, "footer_nav"))
    if mode is None:
        return

    out_dir = text_input(stdscr, t(cfg, "playlist_title"), t(cfg, "outdir_prompt"),
                          t(cfg, "footer_input"), default=cfg["download_dir"])
    if out_dir is None:
        return

    expanded_out = Path(out_dir).expanduser()
    expanded_out.mkdir(parents=True, exist_ok=True)
    template = str(expanded_out / "%(playlist_title)s/%(playlist_index)03d - %(title)s.%(ext)s")
    cargs = cookie_args(cfg)
    if mode == 1:
        cmd = ["yt-dlp", "-x", "--audio-format", cfg["audio_format"], "-o", template, "--yes-playlist", *cargs, url]
    else:
        preset_args = VIDEO_PRESETS[cfg.get("video_preset", "davinci_dnxhr")]["args"]
        cmd = ["yt-dlp", "-f", "bestvideo+bestaudio/best", "-o", template, "--yes-playlist",
               *preset_args, *cargs, url]

    mode_display = t(cfg, "playlist_mode_audio") if mode == 1 else t(cfg, "playlist_mode_video")
    cmd_str = " ".join(cmd)
    subtitle = (f"{t(cfg, 'label_url')} {url}\n"
                f"{t(cfg, 'label_mode')} {mode_display}   {t(cfg, 'label_folder')} {out_dir}\n"
                f"{t(cfg, 'label_cmd')} {cmd_str}")
    confirm = run_menu(stdscr, t(cfg, "confirm_title"), [t(cfg, "confirm_start"), t(cfg, "confirm_cancel")],
                        t(cfg, "footer_nav"), subtitle=subtitle)
    if confirm == 0:
        run_with_log(stdscr, cfg, cmd, op_type="Download Playlist", source=url, target=str(expanded_out))


def screen_info(stdscr, cfg: dict) -> None:
    url = text_input(stdscr, t(cfg, "info_title"), t(cfg, "video_prompt_url"), t(cfg, "footer_input"))
    if not url:
        return

    curses.curs_set(0)
    stdscr.erase()
    h, w = stdscr.getmaxyx()
    draw_header(stdscr, t(cfg, "info_title"), w)
    safe_addstr(stdscr, 3, 4, t(cfg, "info_fetching"), w - 8)
    stdscr.refresh()

    cmd = ["yt-dlp", "--no-playlist", "-j", *cookie_args(cfg), url]
    lines: list[str] = []
    try:
        result = subprocess.run(cmd, check=False, capture_output=True, text=True)
        if result.returncode != 0:
            lines = [t(cfg, "info_error"), result.stderr.strip()[:2000]]
        else:
            data = json.loads(result.stdout)
            heights = sorted({f.get("height") for f in data.get("formats", []) if f.get("height")})
            lines = [
                f"{t(cfg, 'info_label_title')} {data.get('title')}",
                f"{t(cfg, 'info_label_uploader')} {data.get('uploader')}",
                f"{t(cfg, 'info_label_duration')} {data.get('duration_string', data.get('duration'))}",
                f"{t(cfg, 'info_label_views')} {data.get('view_count')}",
                f"{t(cfg, 'info_label_date')} {data.get('upload_date')}",
                f"{t(cfg, 'info_label_quality')} {', '.join(str(h) + 'p' for h in heights) if heights else '-'}",
            ]
    except FileNotFoundError:
        lines = [t(cfg, "info_not_installed")]
    except json.JSONDecodeError:
        lines = [t(cfg, "info_parse_error")]
    show_message(stdscr, t(cfg, "info_title"), lines, t(cfg, "footer_message"))


def screen_subs(stdscr, cfg: dict) -> None:
    url = text_input(stdscr, t(cfg, "subs_title"), t(cfg, "video_prompt_url"), t(cfg, "footer_input"))
    if not url:
        return

    langs = text_input(stdscr, t(cfg, "subs_title"), t(cfg, "subs_langs_prompt"),
                        t(cfg, "footer_input"), default=cfg["sub_langs"])
    if langs is None:
        return

    out_dir = text_input(stdscr, t(cfg, "subs_title"), t(cfg, "outdir_prompt"),
                          t(cfg, "footer_input"), default=cfg["download_dir"])
    if out_dir is None:
        return

    expanded_out = Path(out_dir).expanduser()
    expanded_out.mkdir(parents=True, exist_ok=True)
    cmd = ["yt-dlp", "--skip-download", "--write-subs", "--write-auto-subs", "--sub-langs", langs,
           "-o", str(expanded_out / "%(title)s.%(ext)s"), *cookie_args(cfg), url]

    cmd_str = " ".join(cmd)
    subtitle = (f"{t(cfg, 'label_url')} {url}\n"
                f"{t(cfg, 'label_langs')} {langs}   {t(cfg, 'label_folder')} {out_dir}\n"
                f"{t(cfg, 'label_cmd')} {cmd_str}")
    confirm = run_menu(stdscr, t(cfg, "confirm_title"), [t(cfg, "confirm_start"), t(cfg, "confirm_cancel")],
                        t(cfg, "footer_nav"), subtitle=subtitle)
    if confirm == 0:
        run_with_log(stdscr, cfg, cmd, op_type="Download Subs", source=url, target=str(expanded_out))


def cookies_status_line(cfg: dict) -> str:
    mode = cfg.get("cookies_mode", "none")
    if mode == "file" and cfg.get("cookies_file"):
        n = len(parse_cookies_file(cfg["cookies_file"]))
        return t(cfg, "cookies_status_file", v=n, f=cfg["cookies_file"])
    if mode == "browser" and cfg.get("cookies_browser"):
        return t(cfg, "cookies_status_browser", v=cfg["cookies_browser"])
    return t(cfg, "cookies_status_none")


def screen_cookies(stdscr, cfg: dict) -> None:
    while True:
        path = cfg.get("cookies_file") or DEFAULT_COOKIES_FILE
        items = [
            t(cfg, "cookies_opt_view"),
            t(cfg, "cookies_opt_add"),
            t(cfg, "cookies_opt_edit_file"),
            t(cfg, "cookies_opt_set_path"),
            t(cfg, "cookies_opt_browser"),
            t(cfg, "cookies_opt_disable"),
            t(cfg, "cookies_opt_back"),
        ]
        choice = run_menu(stdscr, t(cfg, "cookies_title"), items, t(cfg, "footer_nav"),
                           subtitle=cookies_status_line(cfg))
        if choice is None or choice == 6:
            return

        if choice == 0:  # view
            cookies = parse_cookies_file(path)
            if not cookies:
                lines = [t(cfg, "cookies_none_found"), "", t(cfg, "cookies_view_note"), path]
            else:
                lines = [f"{c['domain']}  {c['name']} = {c['value'][:40]}" for c in cookies]
                lines += ["", t(cfg, "cookies_view_note"), path]
            show_message(stdscr, t(cfg, "cookies_title"), lines, t(cfg, "footer_message"))

        elif choice == 1:  # add
            domain = text_input(stdscr, t(cfg, "cookies_title"), t(cfg, "cookies_domain_prompt"),
                                 t(cfg, "footer_input"))
            if not domain:
                continue
            name = text_input(stdscr, t(cfg, "cookies_title"), t(cfg, "cookies_name_prompt"),
                               t(cfg, "footer_input"))
            if not name:
                continue
            value = text_input(stdscr, t(cfg, "cookies_title"), t(cfg, "cookies_value_prompt"),
                                t(cfg, "footer_input"))
            if value is None:
                continue
            append_cookie_entry(path, domain, name, value)
            cfg["cookies_mode"] = "file"
            cfg["cookies_file"] = path
            save_config(cfg)
            show_message(stdscr, t(cfg, "cookies_title"), [t(cfg, "cookies_added", v=path)],
                         t(cfg, "footer_message"))

        elif choice == 2:  # edit in $EDITOR
            exp_path = Path(path).expanduser()
            exp_path.parent.mkdir(parents=True, exist_ok=True)
            if not exp_path.exists():
                exp_path.write_text("# Netscape HTTP Cookie File\n# Generated by MogDop's MediaCLI.\n\n")
            editor = (os.environ.get("EDITOR") or
                      shutil.which("nano") or
                      shutil.which("vim") or
                      shutil.which("vi") or
                      shutil.which("notepad") or "notepad")
            run_external(stdscr, [editor, str(exp_path)])
            cfg["cookies_mode"] = "file"
            cfg["cookies_file"] = path
            save_config(cfg)

        elif choice == 3:  # change path
            new_path = text_input(stdscr, t(cfg, "cookies_title"), t(cfg, "cookies_path_prompt"),
                                   t(cfg, "footer_input"), default=path)
            if new_path:
                cfg["cookies_file"] = new_path
                cfg["cookies_mode"] = "file"
                save_config(cfg)

        elif choice == 4:  # browser
            bi = run_menu(stdscr, t(cfg, "cookies_title"), BROWSERS, t(cfg, "footer_nav"),
                          subtitle=t(cfg, "cookies_browser_subtitle"))
            if bi is None:
                continue
            browser = BROWSERS[bi]
            profile = text_input(stdscr, t(cfg, "cookies_title"), t(cfg, "cookies_profile_prompt"),
                                  t(cfg, "footer_input"))
            if profile:
                browser = f"{browser}:{profile}"
            cfg["cookies_mode"] = "browser"
            cfg["cookies_browser"] = browser
            save_config(cfg)

        elif choice == 5:  # disable
            cfg["cookies_mode"] = "none"
            save_config(cfg)


def screen_settings(stdscr, cfg: dict) -> None:
    """Tabbed Settings Menu (General / Conversion / Interface)."""
    current_tab = 0
    current_item = 0

    while True:
        lang = cfg.get("language", "en")
        preset_name = get_preset_name(cfg, cfg.get("video_preset", "davinci_dnxhr"))
        theme_id = cfg.get("theme", "cyan")
        theme_name = THEMES.get(theme_id, THEMES["cyan"])["name_ru"] if lang == "ru" else THEMES.get(theme_id, THEMES["cyan"])["name_en"]
        bg_status = t(cfg, "bg_option_keep") if cfg.get("use_terminal_bg", True) else t(cfg, "bg_option_solid")

        # Define Tab Structure for Settings
        tabs = [
            {
                "title": t(cfg, "tab_set_gen"),
                "items": [
                    t(cfg, "settings_download_dir", v=cfg["download_dir"]),
                    t(cfg, "settings_goal", v=cfg.get("user_goal", "editing")),
                    t(cfg, "settings_proxy", v=(cfg.get("proxy") or "None")),
                    t(cfg, "settings_language", v=("English" if lang == "en" else "Русский")),
                    t(cfg, "settings_cookies", v=cookies_status_line(cfg)),
                ]
            },
            {
                "title": t(cfg, "tab_set_conv"),
                "items": [
                    t(cfg, "settings_preset", v=preset_name),
                    t(cfg, "settings_audio_format", v=cfg["audio_format"]),
                    t(cfg, "settings_sub_langs", v=cfg["sub_langs"]),
                ]
            },
            {
                "title": t(cfg, "tab_set_ui"),
                "items": [
                    t(cfg, "settings_theme", v=theme_name),
                    t(cfg, "settings_bg", v=bg_status),
                    t(cfg, "settings_style", v=cfg.get("progress_style", "blocks")),
                    t(cfg, "settings_reset"),
                    t(cfg, "settings_back"),
                ]
            }
        ]

        t_idx, i_idx = run_menu_tabbed(stdscr, t(cfg, "settings_title"), tabs, t(cfg, "footer_nav"),
                                       start_tab=current_tab, start_index=current_item)

        if t_idx is None:
            return

        current_tab = t_idx
        current_item = i_idx

        # TAB 0: GENERAL
        if current_tab == 0:
            if current_item == 0:  # Dir
                val = text_input(stdscr, t(cfg, "settings_title"), t(cfg, "settings_new_download_dir"),
                                  t(cfg, "footer_input"), default=cfg["download_dir"])
                if val:
                    cfg["download_dir"] = val
                    save_config(cfg)
            elif current_item == 1:  # Goal
                goals = ["editing", "downloading", "audio", "transcoding"]
                goal_labels = [t(cfg, "goal_editing"), t(cfg, "goal_downloading"), t(cfg, "goal_audio"), t(cfg, "goal_transcoding")]
                gi = run_menu(stdscr, t(cfg, "settings_title"), goal_labels, t(cfg, "footer_nav"), subtitle=t(cfg, "settings_choose_goal"))
                if gi is not None:
                    cfg["user_goal"] = goals[gi]
                    save_config(cfg)
            elif current_item == 2:  # Proxy
                pr = text_input(stdscr, t(cfg, "settings_title"), t(cfg, "adv_proxy_prompt"), t(cfg, "footer_input"), default=cfg.get("proxy", ""))
                if pr is not None:
                    cfg["proxy"] = pr.strip()
                    save_config(cfg)
            elif current_item == 3:  # Language
                li = run_menu(stdscr, t(cfg, "settings_title"), ["English", "Русский"], t(cfg, "footer_nav"), subtitle=t(cfg, "settings_choose_language"))
                if li is not None:
                    cfg["language"] = "en" if li == 0 else "ru"
                    save_config(cfg)
            elif current_item == 4:  # Cookies
                screen_cookies(stdscr, cfg)

        # TAB 1: CONVERSION
        elif current_tab == 1:
            if current_item == 0:  # Video preset
                preset_keys = get_ordered_video_preset_keys(cfg)
                preset_labels = [VIDEO_PRESETS[pk]["name_ru"] if lang == "ru" else VIDEO_PRESETS[pk]["name_en"] for pk in preset_keys]
                preset_descs = [VIDEO_PRESETS[pk]["desc_ru"] if lang == "ru" else VIDEO_PRESETS[pk]["desc_en"] for pk in preset_keys]
                pi = run_menu(stdscr, t(cfg, "settings_title"), preset_labels, t(cfg, "footer_nav"), subtitle=t(cfg, "settings_choose_preset"), descriptions=preset_descs)
                if pi is not None:
                    cfg["video_preset"] = preset_keys[pi]
                    save_config(cfg)
            elif current_item == 1:  # Audio format
                fi = run_menu(stdscr, t(cfg, "settings_title"), AUDIO_FORMATS, t(cfg, "footer_nav"), subtitle=t(cfg, "settings_choose_audio_format"))
                if fi is not None:
                    cfg["audio_format"] = AUDIO_FORMATS[fi]
                    save_config(cfg)
            elif current_item == 2:  # Sub langs
                val = text_input(stdscr, t(cfg, "settings_title"), t(cfg, "settings_new_sub_langs"), t(cfg, "footer_input"), default=cfg["sub_langs"])
                if val:
                    cfg["sub_langs"] = val
                    save_config(cfg)

        # TAB 2: INTERFACE
        elif current_tab == 2:
            if current_item == 0:  # Theme
                theme_keys = list(THEMES.keys())
                theme_labels = [THEMES[k]["name_ru"] if lang == "ru" else THEMES[k]["name_en"] for k in theme_keys]
                ti = run_menu(stdscr, t(cfg, "settings_title"), theme_labels, t(cfg, "footer_nav"), subtitle=t(cfg, "settings_choose_theme"))
                if ti is not None:
                    cfg["theme"] = theme_keys[ti]
                    apply_theme(cfg)
                    save_config(cfg)
            elif current_item == 1:  # Terminal BG
                bgi = run_menu(stdscr, t(cfg, "settings_title"), [t(cfg, "bg_option_keep"), t(cfg, "bg_option_solid")], t(cfg, "footer_nav"), subtitle=t(cfg, "bg_option_title"))
                if bgi is not None:
                    cfg["use_terminal_bg"] = (bgi == 0)
                    apply_theme(cfg)
                    save_config(cfg)
            elif current_item == 2:  # Progress Bar Style
                styles = ["blocks", "classic", "dots", "minimal"]
                si = run_menu(stdscr, t(cfg, "settings_title"), styles, t(cfg, "footer_nav"))
                if si is not None:
                    cfg["progress_style"] = styles[si]
                    save_config(cfg)
            elif current_item == 3:  # Reset
                confirm = run_menu(stdscr, t(cfg, "confirm_title"), [t(cfg, "confirm_start"), t(cfg, "confirm_cancel")], t(cfg, "footer_nav"), subtitle=t(cfg, "settings_reset_confirm"))
                if confirm == 0:
                    cfg.clear()
                    cfg.update(reset_config())
                    run_first_run_wizard(stdscr, cfg)
                    return
            elif current_item == 4:  # Back
                return


def screen_update(stdscr, cfg: dict) -> None:
    choice = run_menu(stdscr, t(cfg, "update_title"),
                       [t(cfg, "update_via_pkg"), t(cfg, "update_via_pip"), t(cfg, "update_via_ytdlp"), t(cfg, "update_cancel")],
                       t(cfg, "footer_nav"))
    if choice is None or choice == 3:
        return
    if choice == 0:
        if sys.platform == "win32":
            run_external(stdscr, ["winget", "upgrade", "yt-dlp"])
        elif sys.platform == "darwin":
            run_external(stdscr, ["brew", "upgrade", "yt-dlp"])
        elif shutil.which("pacman"):
            run_external(stdscr, ["sudo", "pacman", "-Syu", "yt-dlp"])
        elif shutil.which("apt"):
            run_external(stdscr, ["sudo", "apt", "update"])
            run_external(stdscr, ["sudo", "apt", "install", "--only-upgrade", "yt-dlp"])
        elif shutil.which("dnf"):
            run_external(stdscr, ["sudo", "dnf", "upgrade", "yt-dlp"])
        else:
            run_with_log(stdscr, cfg, ["python", "-m", "pip", "install", "--user", "-U", "yt-dlp"])
    elif choice == 1:
        run_with_log(stdscr, cfg, ["python", "-m", "pip", "install", "--user", "-U", "yt-dlp"])
    elif choice == 2:
        run_with_log(stdscr, cfg, ["yt-dlp", "-U"])


# ==========================================================================
# Main menu (Tabbed Layout)
# ==========================================================================

MAIN_TABS = [
    {
        "id": "online",
        "title_key": "tab_online",
        "keys": [("menu_video", screen_video), ("menu_audio", screen_audio), ("menu_playlist", screen_playlist), ("menu_subs", screen_subs), ("menu_info", screen_info)]
    },
    {
        "id": "local",
        "title_key": "tab_local",
        "keys": [("menu_convert", screen_convert), ("menu_batch_convert", screen_batch_convert), ("menu_trim", screen_trim), ("menu_probe", screen_probe)]
    },
    {
        "id": "system",
        "title_key": "tab_system",
        "keys": [("menu_history", screen_history), ("menu_cookies", screen_cookies), ("menu_settings", screen_settings), ("menu_update", screen_update), ("menu_exit", None)]
    }
]


def main_tui(stdscr) -> None:
    cfg = load_config()
    apply_theme(cfg)

    # First Run Wizard check
    if not cfg.get("first_run_completed", False):
        run_first_run_wizard(stdscr, cfg)

    # Auto Dependency check & Arch Linux prompt
    missing = missing_dependencies()
    if missing:
        missing_bins = [bin_ for bin_, _ in missing]
        lines = [t(cfg, "deps_missing_header"), ""]
        for bin_ in missing_bins:
            lines.append(f"  - {bin_}")

        if shutil.which("pacman"):
            lines += ["", t(cfg, "deps_arch_install_prompt")]
            choice = run_menu(stdscr, t(cfg, "deps_title"), [t(cfg, "confirm_start"), t(cfg, "confirm_cancel")], t(cfg, "footer_nav"), subtitle="\n".join(lines))
            if choice == 0:
                run_external(stdscr, ["sudo", "pacman", "-S", "--noconfirm", *missing_bins])
        else:
            lines += ["", t(cfg, "deps_install_header"), "  " + get_install_command(missing_bins), "", t(cfg, "deps_note")]
            show_message(stdscr, t(cfg, "deps_title"), lines, t(cfg, "footer_message"))

    current_tab = 0
    current_item = 0

    while True:
        # Build Tabbed structure with i18n
        tabs = []
        for tab in MAIN_TABS:
            tabs.append({
                "title": t(cfg, tab["title_key"]),
                "items": [t(cfg, k[0]) for k in tab["keys"]]
            })

        t_idx, i_idx = run_menu_tabbed(stdscr, t(cfg, "app_title"), tabs, t(cfg, "footer_nav"),
                                       start_tab=current_tab, start_index=current_item, ascii_art=ASCII_ART)

        if t_idx is None:
            break

        current_tab = t_idx
        current_item = i_idx

        key, func = MAIN_TABS[current_tab]["keys"][current_item]
        if key == "menu_exit":
            return
        if func:
            func(stdscr, cfg)


# ==========================================================================
# Classic (non-curses) fallback
# ==========================================================================

def run(cmd: list[str]) -> int:
    print("[cmd]", " ".join(cmd))
    try:
        return subprocess.run(cmd, check=False).returncode
    except FileNotFoundError:
        print("[!] Tool is not installed.")
        return 1
    except KeyboardInterrupt:
        print("\n[!] Interrupted.")
        return 130


def classic_menu() -> int:
    cfg = load_config()
    print("=== MogDop's MediaCLI (classic menu, curses unavailable) ===")
    print("1) Video  2) Audio  3) Playlist  4) Convert local file  5) Info  6) Subtitles  7) Update  0) Exit")
    try:
        choice = input("Choice: ").strip()
    except (KeyboardInterrupt, EOFError):
        return 0
    if choice == "0" or not choice:
        return 0
    if choice == "7":
        return run(["python", "-m", "pip", "install", "-U", "yt-dlp"])

    if choice == "4":
        try:
            inp = input("Enter path to file: ").strip()
            if not inp or not Path(inp).expanduser().exists():
                print("File not found.")
                return 1
            print("1) DaVinci DNxHR HQ  2) DaVinci ProRes HQ  3) DaVinci H.264+PCM  4) Standard MP4  5) WAV  6) MP3  7) Custom")
            p_sel = input("Preset [1]: ").strip() or "1"
            p_map = {"1": 0, "2": 1, "3": 2, "4": 3, "5": 4, "6": 5, "7": 6}
            preset = CONVERT_PRESETS[p_map.get(p_sel, 0)]
            inp_path = Path(inp).expanduser()

            if preset["id"] == "custom":
                ext = input("Custom extension [mov]: ").strip() or "mov"
                flags_str = input("Custom FFmpeg flags [-c:v libx264 -c:a pcm_s16le]: ").strip() or "-c:v libx264 -c:a pcm_s16le"
                flags = flags_str.split()
                out_file = inp_path.parent / f"{inp_path.stem}_custom.{ext}"
            else:
                flags = preset["ffmpeg_flags"]
                out_file = inp_path.parent / f"{inp_path.stem}{preset['suffix']}.{preset['ext']}"

            cmd = ["ffmpeg", "-y", "-i", str(inp_path), *flags, str(out_file)]
            return run(cmd)
        except (KeyboardInterrupt, EOFError):
            return 0

    try:
        url = input("URL: ").strip()
        if not url:
            return 1
        out = input(f"Folder [{cfg['download_dir']}]: ").strip() or cfg["download_dir"]
    except (KeyboardInterrupt, EOFError):
        return 0

    exp_out = Path(out).expanduser()
    exp_out.mkdir(parents=True, exist_ok=True)
    cargs = cookie_args(cfg)

    if choice == "1":
        q = input("Quality, e.g. 1080 (Enter = best): ").strip()
        fmt = f"bestvideo[height<={q}]+bestaudio/best[height<={q}]" if q else "bestvideo+bestaudio/best"
        preset_args = VIDEO_PRESETS[cfg.get("video_preset", "davinci_dnxhr")]["args"]
        cmd = ["yt-dlp", "-f", fmt, "-o", str(exp_out / "%(title)s.%(ext)s"),
               *preset_args, *cargs, url]
    elif choice == "2":
        f = input(f"Audio format [{cfg['audio_format']}]: ").strip() or cfg["audio_format"]
        cmd = ["yt-dlp", "-x", "--audio-format", f, "-o", str(exp_out / "%(title)s.%(ext)s"), *cargs, url]
    elif choice == "3":
        template = str(exp_out / "%(playlist_title)s/%(playlist_index)03d - %(title)s.%(ext)s")
        preset_args = VIDEO_PRESETS[cfg.get("video_preset", "davinci_dnxhr")]["args"]
        cmd = ["yt-dlp", "-f", "bestvideo+bestaudio/best", "-o", template, "--yes-playlist",
               *preset_args, *cargs, url]
    elif choice == "5":
        cmd = ["yt-dlp", "--no-playlist", "-j", *cargs, url]
    elif choice == "6":
        cmd = ["yt-dlp", "--skip-download", "--write-subs", "--write-auto-subs",
               "--sub-langs", cfg["sub_langs"], "-o", str(exp_out / "%(title)s.%(ext)s"), *cargs, url]
    else:
        print("Invalid choice.")
        return 1
    return run(cmd)


# ==========================================================================
# Command-line subcommands (scripting)
# ==========================================================================

def _resolve_cookie_args(args: argparse.Namespace) -> list[str]:
    if getattr(args, "cookies", None):
        return ["--cookies", str(Path(args.cookies).expanduser())]
    if getattr(args, "cookies_from_browser", None):
        return ["--cookies-from-browser", args.cookies_from_browser]
    return cookie_args(load_config())


def cli_video(args: argparse.Namespace) -> int:
    cfg = load_config()
    out_dir = Path(args.output).expanduser()
    out_dir.mkdir(parents=True, exist_ok=True)
    fmt = f"bestvideo[height<={args.quality}]+bestaudio/best[height<={args.quality}]" if args.quality else "bestvideo+bestaudio/best"

    preset_key = PRESET_CLI_MAP.get(args.preset, cfg.get("video_preset", "davinci_dnxhr"))
    if preset_key == "custom":
        custom_ext = args.custom_ext or "mp4"
        custom_flags = args.custom_flags or "-c:v libx264 -c:a aac"
        preset_args = ["--recode-video", custom_ext, "--postprocessor-args", f"ffmpeg:{custom_flags}"]
    else:
        preset_args = VIDEO_PRESETS[preset_key]["args"]

    cmd = ["yt-dlp"]
    if args.all_audio:
        cmd += ["--audio-multistreams"]
    if args.sponsorblock:
        cmd += ["--sponsorblock-remove", args.sponsorblock]
    if getattr(args, "proxy", None):
        cmd += ["--proxy", args.proxy]

    cmd += ["-f", fmt, "-o", str(out_dir / "%(title)s.%(ext)s"),
            *preset_args, *_resolve_cookie_args(args), args.url]
    if args.subs:
        cmd += ["--write-subs", "--sub-langs", args.subs]
    return run(cmd)


def cli_convert(args: argparse.Namespace) -> int:
    inp_path = Path(args.input).expanduser()
    if not inp_path.exists():
        print(f"[!] Input file '{inp_path}' does not exist.")
        return 1

    out_dir = Path(args.output_dir).expanduser() if args.output_dir else inp_path.parent
    out_dir.mkdir(parents=True, exist_ok=True)

    if args.preset == "custom":
        ext = args.custom_ext or "mov"
        flags = (args.custom_flags or "-c:v libx264 -c:a pcm_s16le").split()
        suffix = "_custom"
    else:
        preset_dict = next((p for p in CONVERT_PRESETS if p["id"] == args.preset), CONVERT_PRESETS[0])
        ext = preset_dict["ext"]
        flags = preset_dict["ffmpeg_flags"]
        suffix = preset_dict["suffix"]

    out_file = out_dir / f"{inp_path.stem}{suffix}.{ext}"
    cmd = ["ffmpeg", "-y", "-i", str(inp_path), *flags, str(out_file)]
    return run(cmd)


def cli_trim(args: argparse.Namespace) -> int:
    inp_path = Path(args.input).expanduser()
    if not inp_path.exists():
        print(f"[!] Input file '{inp_path}' does not exist.")
        return 1

    out_file = inp_path.parent / f"{inp_path.stem}_trimmed{inp_path.suffix}"
    cmd = ["ffmpeg", "-y", "-ss", args.start]
    if args.end:
        cmd += ["-to", args.end]
    cmd += ["-i", str(inp_path)]

    if args.copy:
        cmd += ["-c", "copy", str(out_file)]
    else:
        cmd += ["-c:v", "libx264", "-c:a", "aac", str(out_file)]

    return run(cmd)


def cli_probe(args: argparse.Namespace) -> int:
    inp_path = Path(args.input).expanduser()
    if not inp_path.exists():
        print(f"[!] Input file '{inp_path}' does not exist.")
        return 1

    cmd = ["ffprobe", "-hide_banner", "-i", str(inp_path)]
    return run(cmd)


def cli_audio(args: argparse.Namespace) -> int:
    out_dir = Path(args.output).expanduser()
    out_dir.mkdir(parents=True, exist_ok=True)
    cmd = ["yt-dlp", "-x", "--audio-format", args.format, "--audio-quality", "0",
           "-o", str(out_dir / "%(title)s.%(ext)s"), *_resolve_cookie_args(args), args.url]
    return run(cmd)


def cli_playlist(args: argparse.Namespace) -> int:
    cfg = load_config()
    out_dir = Path(args.output).expanduser()
    out_dir.mkdir(parents=True, exist_ok=True)
    template = str(out_dir / "%(playlist_title)s/%(playlist_index)03d - %(title)s.%(ext)s")
    cargs = _resolve_cookie_args(args)
    if args.audio_only:
        cmd = ["yt-dlp", "-x", "--audio-format", cfg["audio_format"], "-o", template, "--yes-playlist", *cargs, args.url]
    else:
        preset_key = PRESET_CLI_MAP.get(args.preset, cfg.get("video_preset", "davinci_dnxhr"))
        preset_args = VIDEO_PRESETS[preset_key]["args"]
        cmd = ["yt-dlp", "-f", "bestvideo+bestaudio/best", "-o", template, "--yes-playlist",
               *preset_args, *cargs, args.url]
    return run(cmd)


def cli_info(args: argparse.Namespace) -> int:
    cmd = ["yt-dlp", "--no-playlist", "-j", *_resolve_cookie_args(args), args.url]
    print("[cmd]", " ".join(cmd))
    try:
        result = subprocess.run(cmd, check=False, capture_output=True, text=True)
    except FileNotFoundError:
        print("[!] yt-dlp is not installed.")
        return 1
    if result.returncode != 0:
        print(result.stderr.strip())
        return result.returncode
    data = json.loads(result.stdout)
    print(f"Title:    {data.get('title')}")
    print(f"Uploader: {data.get('uploader')}")
    print(f"Duration: {data.get('duration_string', data.get('duration'))}")
    print(f"Views:    {data.get('view_count')}")
    heights = sorted({f.get("height") for f in data.get("formats", []) if f.get("height")})
    if heights:
        print(f"Quality:  {', '.join(str(h) + 'p' for h in heights)}")
    return 0


def cli_subs(args: argparse.Namespace) -> int:
    out_dir = Path(args.output).expanduser()
    out_dir.mkdir(parents=True, exist_ok=True)
    cmd = ["yt-dlp", "--skip-download", "--write-subs", "--write-auto-subs", "--sub-langs", args.langs,
           "-o", str(out_dir / "%(title)s.%(ext)s"), *_resolve_cookie_args(args), args.url]
    return run(cmd)


def cli_update(_args: argparse.Namespace) -> int:
    if sys.platform == "win32":
        return run(["winget", "upgrade", "yt-dlp"])
    elif sys.platform == "darwin":
        return run(["brew", "upgrade", "yt-dlp"])
    elif shutil.which("pacman"):
        return run(["sudo", "pacman", "-Syu", "yt-dlp"])
    elif shutil.which("apt"):
        return run(["sudo", "apt", "update"]) or run(["sudo", "apt", "install", "--only-upgrade", "yt-dlp"])
    return run(["python", "-m", "pip", "install", "--user", "-U", "yt-dlp"])


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="mediacli", description="MogDop's MediaCLI — yt-dlp & FFmpeg toolkit")
    sub = parser.add_subparsers(dest="command")

    cookie_parent = argparse.ArgumentParser(add_help=False)
    cookie_parent.add_argument("--cookies", default=None, help="path to a Netscape-format cookies.txt file")
    cookie_parent.add_argument("--cookies-from-browser", default=None,
                                help="e.g. firefox, chrome, chrome:Default")

    p_video = sub.add_parser("video", help="Download video", parents=[cookie_parent])
    p_video.add_argument("url")
    p_video.add_argument("-q", "--quality", default="")
    p_video.add_argument("-p", "--preset", default="davinci-dnxhr", choices=list(PRESET_CLI_MAP.keys()),
                         help="FFmpeg export preset (e.g. davinci-dnxhr, standard-mp4, custom)")
    p_video.add_argument("--all-audio", action="store_true", help="Download all available audio streams")
    p_video.add_argument("--sponsorblock", default=None, help="Remove sponsors (e.g. sponsor, selfpromo)")
    p_video.add_argument("--proxy", default=None, help="Proxy URL (e.g. socks5://127.0.0.1:1080)")
    p_video.add_argument("--custom-ext", default="mp4", help="Extension for custom preset")
    p_video.add_argument("--custom-flags", default="-c:v libx264 -c:a aac", help="Flags for custom preset")
    p_video.add_argument("-o", "--output", default=DEFAULT_DOWNLOAD_DIR)
    p_video.add_argument("-s", "--subs", default=None)
    p_video.set_defaults(func=cli_video)

    p_convert = sub.add_parser("convert", help="Convert local media file with FFmpeg")
    p_convert.add_argument("input", help="Path to local file")
    p_convert.add_argument("-p", "--preset", default="davinci_dnxhr_hq",
                           choices=[p["id"] for p in CONVERT_PRESETS],
                           help="Target conversion preset")
    p_convert.add_argument("--custom-ext", default="mov", help="Extension for custom preset")
    p_convert.add_argument("--custom-flags", default="-c:v libx264 -c:a pcm_s16le", help="Flags for custom preset")
    p_convert.add_argument("-o", "--output-dir", default=None, help="Destination directory")
    p_convert.set_defaults(func=cli_convert)

    p_trim = sub.add_parser("trim", help="Trim media file by time")
    p_trim.add_argument("input", help="Path to input file")
    p_trim.add_argument("-s", "--start", default="00:00:00", help="Start time")
    p_trim.add_argument("-e", "--end", default=None, help="End time")
    p_trim.add_argument("--copy", action="store_true", help="Stream copy (no re-encoding)")
    p_trim.set_defaults(func=cli_trim)

    p_probe = sub.add_parser("probe", help="Inspect local media file with FFprobe")
    p_probe.add_argument("input", help="Path to input file")
    p_probe.set_defaults(func=cli_probe)

    p_audio = sub.add_parser("audio", help="Download audio only", parents=[cookie_parent])
    p_audio.add_argument("url")
    p_audio.add_argument("-f", "--format", default="mp3", choices=AUDIO_FORMATS)
    p_audio.add_argument("-o", "--output", default=DEFAULT_DOWNLOAD_DIR)
    p_audio.set_defaults(func=cli_audio)

    p_playlist = sub.add_parser("playlist", help="Download a playlist", parents=[cookie_parent])
    p_playlist.add_argument("url")
    p_playlist.add_argument("-p", "--preset", default="davinci-dnxhr", choices=list(PRESET_CLI_MAP.keys()))
    p_playlist.add_argument("-o", "--output", default=DEFAULT_DOWNLOAD_DIR)
    p_playlist.add_argument("--audio-only", action="store_true")
    p_playlist.set_defaults(func=cli_playlist)

    p_info = sub.add_parser("info", help="Show video info")
    p_info.add_argument("url")
    p_info.set_defaults(func=cli_info)

    p_subs = sub.add_parser("subs", help="Download subtitles only", parents=[cookie_parent])
    p_subs.add_argument("url")
    p_subs.add_argument("-l", "--langs", default="ru,en")
    p_subs.add_argument("-o", "--output", default=DEFAULT_DOWNLOAD_DIR)
    p_subs.set_defaults(func=cli_subs)

    p_update = sub.add_parser("update", help="Update yt-dlp")
    p_update.set_defaults(func=cli_update)

    return parser


def main() -> int:
    setup_signal_handlers()

    parser = build_parser()
    args = parser.parse_args()

    if args.command is not None:
        return args.func(args)

    try:
        locale.setlocale(locale.LC_ALL, "")
    except locale.Error:
        pass

    if curses is not None and sys.stdout.isatty() and sys.stdin.isatty():
        try:
            curses.wrapper(main_tui)
            return 0
        except curses.error:
            print("[!] Curses is not available in this terminal, falling back to classic menu.\n")

    if sys.platform == "win32" and curses is None:
        print("[!] Note: For full TUI menu on Windows, run: pip install windows-curses\n")

    return classic_menu()


if __name__ == "__main__":
    sys.exit(main())