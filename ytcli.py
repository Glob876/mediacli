"""
MogDop's MediaCLI — Advanced Console Media Suite around yt-dlp & FFmpeg
with Tabbed/Split TUI, Global Embedded Overlay Terminal, Background Queue,
and Robust Multi-Platform Preset Engine.
"""

import argparse
import json
import locale
import os
import queue
import re
import select
import shutil
import signal
import subprocess
import sys
import threading
import time
from pathlib import Path

try:
    import curses
except ImportError:
    curses = None

# ==========================================================================
# Cross-Platform Directory & Path Resolvers
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


def parse_user_path(raw_path: str) -> Path:
    if not raw_path:
        return Path(".")
    s = raw_path.strip()
    if (s.startswith('"') and s.endswith('"')) or (s.startswith("'") and s.endswith("'")):
        s = s[1:-1].strip()
    s = s.replace('"', '').replace("'", '')
    return Path(s).expanduser()


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
    "cookies_browser": "",
    "video_preset": "davinci_dnxhr",
    "theme": "cyan",
    "use_terminal_bg": True,
    "proxy_mode": "system",       # "system" | "custom" | "none"
    "proxy_url": "",
    "progress_style": "blocks",   # "blocks" | "classic" | "dots" | "minimal"
    "bg_queue_max": 3,
    "notify_bell": True,
    "auto_check_deps": True,
    "default_editor": "",
    "download_presets": [],
    "default_download_preset": None,
    "preset_defaults": None,
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

THEMES = {
    "cyan": {"id": "cyan", "name_en": "Arch Cyan (Default)", "name_ru": "Arch Cyan (По умолчанию)", "hl_fg": 0, "hl_bg": 6},
    "nord": {"id": "nord", "name_en": "Nord Blue", "name_ru": "Nord Blue", "hl_fg": 0, "hl_bg": 4},
    "matrix": {"id": "matrix", "name_en": "Matrix Green", "name_ru": "Matrix Green", "hl_fg": 0, "hl_bg": 2},
    "dracula": {"id": "dracula", "name_en": "Dracula Magenta", "name_ru": "Dracula Magenta", "hl_fg": 0, "hl_bg": 5},
    "gruvbox": {"id": "gruvbox", "name_en": "Gruvbox Yellow", "name_ru": "Gruvbox Yellow", "hl_fg": 0, "hl_bg": 3},
    "fire": {"id": "fire", "name_en": "Fire Red", "name_ru": "Fire Red", "hl_fg": 0, "hl_bg": 1},
    "classic": {"id": "classic", "name_en": "Classic High Contrast (White)", "name_ru": "Классический (Белый)", "hl_fg": 0, "hl_bg": 7}
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
        curses.init_pair(1, theme["hl_fg"], theme["hl_bg"])
        curses.init_pair(2, curses.COLOR_BLACK, curses.COLOR_WHITE)
        curses.init_pair(3, curses.COLOR_YELLOW, -1)
        curses.init_pair(4, curses.COLOR_GREEN, -1)
        curses.init_pair(5, curses.COLOR_CYAN, -1)
        HL_ATTR = curses.color_pair(1)
    except curses.error:
        HL_ATTR = curses.A_REVERSE

# ==========================================================================
# Presets and Conversion Definitions
# ==========================================================================

VIDEO_PRESETS = {
    "davinci_dnxhr": {
        "id": "davinci_dnxhr",
        "name_en": "DaVinci Resolve: DNxHR HQ + PCM Audio (.mov) [RECOMMENDED]",
        "name_ru": "DaVinci Resolve: DNxHR HQ + PCM Аудио (.mov) [РЕКОМЕНДУЕТСЯ]",
        "desc_en": "Converts video to Avid DNxHR HQ with PCM 16-bit audio. Optimal for DaVinci Resolve.",
        "desc_ru": "Конвертирует в Avid DNxHR HQ с PCM 16-бит аудио. Идеально для DaVinci Resolve.",
        "args": ["--recode-video", "mov", "--postprocessor-args", "ffmpeg:-c:v dnxhd -profile:v dnxhr_hq -c:a pcm_s16le"]
    },
    "davinci_prores": {
        "id": "davinci_prores",
        "name_en": "DaVinci Resolve: Apple ProRes 422 + PCM Audio (.mov)",
        "name_ru": "DaVinci Resolve: Apple ProRes 422 + PCM Аудио (.mov)",
        "desc_en": "Apple ProRes HQ video codec with uncompressed PCM audio in MOV container.",
        "desc_ru": "Видеокодек Apple ProRes HQ с несжатым PCM аудио в MOV контейнере.",
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
        "desc_en": "Universal MP4 container with H.264 video and AAC audio. High compatibility.",
        "desc_ru": "Универсальный MP4 с H.264 видео и AAC аудио. Максимальная совместимость.",
        "args": ["--recode-video", "mp4", "--postprocessor-args", "ffmpeg:-c:v libx264 -c:a aac"]
    },
    "hevc_mp4": {
        "id": "hevc_mp4",
        "name_en": "High Efficiency MP4 (H.265 / HEVC + AAC)",
        "name_ru": "Высокоэффективный MP4 (H.265 / HEVC + AAC)",
        "desc_en": "H.265/HEVC video codec. Up to 50% smaller file size with high visual quality.",
        "desc_ru": "Видеокодек H.265/HEVC. До 50% меньший размер файла при высоком качестве.",
        "args": ["--recode-video", "mp4", "--postprocessor-args", "ffmpeg:-c:v libx265 -c:a aac"]
    },
    "webm_vp9": {
        "id": "webm_vp9",
        "name_en": "WebM (VP9 + Opus)",
        "name_ru": "WebM (VP9 + Opus)",
        "desc_en": "Open WebM media format with VP9 video and Opus audio. Perfect for web browsers.",
        "desc_ru": "Открытый веб-формат WebM с видео VP9 и аудио Opus. Идеально для браузеров.",
        "args": ["--recode-video", "webm", "--postprocessor-args", "ffmpeg:-c:v libvpx-vp9 -c:a libopus"]
    },
    "mkv_av1": {
        "id": "mkv_av1",
        "name_en": "Modern MKV (AV1 + AAC)",
        "name_ru": "Современный MKV (AV1 + AAC)",
        "desc_en": "Next-generation AV1 video codec in Matroska MKV container. Ultra high efficiency.",
        "desc_ru": "Видеокодек нового поколения AV1 в контейнере MKV. Высокая эффективность сжатия.",
        "args": ["--recode-video", "mkv", "--postprocessor-args", "ffmpeg:-c:v libsvtav1 -c:a aac"]
    },
    "audio_mp3": {
        "id": "audio_mp3",
        "name_en": "Audio Only: MP3 320 kbps",
        "name_ru": "Только аудио: MP3 320 кбит/с",
        "desc_en": "Extracts audio stream into high quality 320 kbps MP3 format.",
        "desc_ru": "Извлекает звуковую дорожку в MP3 320 кбит/с.",
        "args": ["-x", "--audio-format", "mp3", "--audio-quality", "0"]
    },
    "audio_flac": {
        "id": "audio_flac",
        "name_en": "Audio Only: FLAC Lossless",
        "name_ru": "Только аудио: FLAC без потерь",
        "desc_en": "Extracts audio stream into lossless FLAC audio format.",
        "desc_ru": "Извлекает звуковую дорожку в сжатый формат без потерь FLAC.",
        "args": ["-x", "--audio-format", "flac"]
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
    "hevc-mp4": "hevc_mp4",
    "webm-vp9": "webm_vp9",
    "mkv-av1": "mkv_av1",
    "custom": "custom",
}

CONVERT_PRESETS = [
    {
        "id": "davinci_dnxhr_hq",
        "name_en": "DaVinci Resolve: DNxHR HQ + PCM (.mov)",
        "name_ru": "DaVinci Resolve: DNxHR HQ + PCM (.mov)",
        "desc_en": "Avid DNxHR HQ video with 16-bit PCM audio in MOV.",
        "desc_ru": "Кодек Avid DNxHR HQ с несжатым PCM 16-бит аудио в MOV.",
        "ext": "mov",
        "suffix": "_dnxhr",
        "ffmpeg_flags": ["-c:v", "dnxhd", "-profile:v", "dnxhr_hq", "-c:a", "pcm_s16le"]
    },
    {
        "id": "davinci_prores",
        "name_en": "DaVinci Resolve: Apple ProRes 422 + PCM (.mov)",
        "name_ru": "DaVinci Resolve: Apple ProRes 422 + PCM (.mov)",
        "desc_en": "Apple ProRes HQ video codec with PCM 16-bit audio.",
        "desc_ru": "Видеокодек Apple ProRes HQ с несжатым PCM 16-бит аудио.",
        "ext": "mov",
        "suffix": "_prores",
        "ffmpeg_flags": ["-c:v", "prores_ks", "-profile:v", "3", "-c:a", "pcm_s16le"]
    },
    {
        "id": "standard_mp4",
        "name_en": "Standard MP4 (H.264 + AAC)",
        "name_ru": "Стандартный MP4 (H.264 + AAC)",
        "desc_en": "Universal MP4 container with H.264 video and AAC audio.",
        "desc_ru": "Универсальный MP4 с H.264 видео и AAC аудио.",
        "ext": "mp4",
        "suffix": "_mp4",
        "ffmpeg_flags": ["-c:v", "libx264", "-c:a", "aac", "-b:a", "192k"]
    },
    {
        "id": "fast_compress",
        "name_en": "Fast Video Compress (H.264 CRF 28)",
        "name_ru": "Быстрое сжатие видео (H.264 CRF 28)",
        "desc_en": "Reduces file size dramatically for easy web sharing.",
        "desc_ru": "Сильно уменьшает размер файла для быстрой отправки в сеть.",
        "ext": "mp4",
        "suffix": "_compressed",
        "ffmpeg_flags": ["-c:v", "libx264", "-crf", "28", "-preset", "faster", "-c:a", "aac", "-b:a", "128k"]
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
        "name_en": "Extract Audio: MP3 320 kbps (.mp3)",
        "name_ru": "Извлечь аудио: MP3 320 кбит/с (.mp3)",
        "desc_en": "Extracts audio track into high-bitrate MP3 audio file.",
        "desc_ru": "Извлекает аудиодорожку в MP3 файл 320 кбит/с.",
        "ext": "mp3",
        "suffix": "_audio",
        "ffmpeg_flags": ["-vn", "-ab", "320k"]
    },
    {
        "id": "custom",
        "name_en": "Custom FFmpeg command...",
        "name_ru": "Свой пресет FFmpeg...",
        "desc_en": "Specify custom file extension and flags manually.",
        "desc_ru": "Задать собственное расширение и флаги FFmpeg вручную.",
        "ext": "mov",
        "suffix": "_custom",
        "ffmpeg_flags": []
    }
]

def get_ordered_video_preset_keys(cfg: dict) -> list[str]:
    goal = cfg.get("user_goal", "editing")
    if goal == "downloading":
        return ["standard_mp4", "hevc_mp4", "default", "webm_vp9", "mkv_av1", "davinci_dnxhr", "davinci_prores", "audio_mp3", "audio_flac", "custom"]
    elif goal == "audio":
        return ["audio_mp3", "audio_flac", "standard_mp4", "default", "davinci_dnxhr", "custom"]
    return ["davinci_dnxhr", "davinci_prores", "davinci_h264", "standard_mp4", "hevc_mp4", "default", "webm_vp9", "mkv_av1", "audio_mp3", "audio_flac", "custom"]

def get_ordered_convert_presets(cfg: dict) -> list[dict]:
    goal = cfg.get("user_goal", "editing")
    if goal in ("downloading", "transcoding"):
        order = ["standard_mp4", "fast_compress", "davinci_dnxhr_hq", "davinci_prores", "audio_mp3", "audio_wav", "custom"]
    elif goal == "audio":
        order = ["audio_mp3", "audio_wav", "standard_mp4", "davinci_dnxhr_hq", "custom"]
    else:
        order = ["davinci_dnxhr_hq", "davinci_prores", "standard_mp4", "fast_compress", "audio_wav", "audio_mp3", "custom"]
    p_dict = {p["id"]: p for p in CONVERT_PRESETS}
    return [p_dict[pid] for pid in order if pid in p_dict]

def get_proxy_args(cfg: dict, override_mode: str = None, override_url: str = None) -> list[str]:
    mode = override_mode if override_mode is not None else cfg.get("proxy_mode", "system")
    url = override_url if override_url is not None else cfg.get("proxy_url", "")
    if mode == "custom" and url and url.strip():
        return ["--proxy", url.strip()]
    elif mode == "none":
        return ["--proxy", ""]
    return []

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
    entries = entries[:50]
    HISTORY_PATH.write_text(json.dumps(entries, indent=2, ensure_ascii=False), encoding="utf-8")

# ==========================================================================
# Dynamic Stage Parser Engine (Requirement 5)
# ==========================================================================

class DynamicStageDetector:
    """Extracts a concise, single-line description of the active stage from live process stdout."""

    PATTERNS = [
        (re.compile(r'\[(youtube|vk|twitch|generic|bilibili|soundcloud)[^\]]*\]\s*([^:]+):\s*(Downloading.*|Extracting.*)'),
         lambda m: f"[{m.group(1).upper()}] {m.group(3)}"),
        (re.compile(r'\[SponsorBlock\]\s*(.*)'), lambda m: f"[SponsorBlock] {m.group(1)}"),
        (re.compile(r'\[Merger\]\s*(.*)'), lambda m: f"[Merger] {m.group(1)}"),
        (re.compile(r'\[VideoConvertor\]\s*(.*)'), lambda m: f"[Convert] {m.group(1)}"),
        (re.compile(r'\[ExtractAudio\]\s*(.*)'), lambda m: f"[AudioExtract] {m.group(1)}"),
        (re.compile(r'\[Metadata\]\s*(.*)'), lambda m: f"[Metadata] {m.group(1)}"),
        (re.compile(r'\[EmbedThumbnail\]\s*(.*)'), lambda m: f"[Thumbnail] {m.group(1)}"),
        (re.compile(r'\[ThumbnailsConvertor\]\s*(.*)'), lambda m: f"[Thumbnail] {m.group(1)}"),
        (re.compile(r'\[ModifyChapters\]\s*(.*)'), lambda m: f"[Chapters] {m.group(1)}"),
        (re.compile(r'\[download\]\s+Destination:\s*(.+)'), lambda m: f"[Download] Saving {Path(m.group(1)).name}"),
        (re.compile(r'\[download\]\s+(\d+(?:\.\d+)?%).*at\s+([^\s]+)\s+ETA\s+([^\s]+)'),
         lambda m: f"[Download] {m.group(1)} (Speed: {m.group(2)}, ETA: {m.group(3)})"),
        (re.compile(r'frame=\s*(\d+)\s+fps=\s*(\d+).*time=([^\s]+).*speed=\s*([^\s]+)'),
         lambda m: f"[FFmpeg] Frame {m.group(1)} ({m.group(2)} fps, Time: {m.group(3)}, Speed: {m.group(4)})"),
        (re.compile(r'size=\s*([^\s]+)\s+time=([^\s]+)\s+bitrate=\s*([^\s]+)'),
         lambda m: f"[FFmpeg] Time: {m.group(2)} ({m.group(1)}, Bitrate: {m.group(3)})"),
    ]

    @classmethod
    def detect(cls, line: str, current_stage: str) -> str:
        s = line.strip()
        if not s:
            return current_stage
        for regex, formatter in cls.PATTERNS:
            m = regex.search(s)
            if m:
                try:
                    return formatter(m)
                except Exception:
                    pass
        if s.startswith("ERROR:"):
            return f"[Error] {s[6:].strip()[:70]}"
        elif s.startswith("WARNING:"):
            return f"[Warning] {s[8:].strip()[:70]}"
        return current_stage

# ==========================================================================
# Background Queue Manager (Requirement 6)
# ==========================================================================

class BackgroundTask:
    def __init__(self, task_id: int, cmd: list[str], title: str, source: str = "", target: str = "", proc=None):
        self.id = task_id
        self.cmd = cmd
        self.title = title
        self.source = source
        self.target = target
        self.proc = proc
        self.status = "running" if proc else "queued"
        self.stage = "Initializing..."
        self.pct = 0.0
        self.log_lines = []
        self.started_at = time.time()
        self.finished_at = None
        self.exit_code = None

class BackgroundQueueManager:
    def __init__(self, max_tasks: int = 3):
        self.max_tasks = max_tasks
        self.tasks: list[BackgroundTask] = []
        self.lock = threading.Lock()
        self.next_id = 1
        self.worker_thread = threading.Thread(target=self._worker_loop, daemon=True)
        self.worker_thread.start()

    def add_running_process(self, proc: subprocess.Popen, cmd: list[str], title: str, source: str, target: str, initial_logs: list[str]) -> BackgroundTask:
        with self.lock:
            task = BackgroundTask(self.next_id, cmd, title, source, target, proc=proc)
            self.next_id += 1
            task.log_lines = list(initial_logs)
            task.status = "running"
            self.tasks.append(task)
            t = threading.Thread(target=self._monitor_proc, args=(task,), daemon=True)
            t.start()
            return task

    def enqueue_cmd(self, cmd: list[str], title: str, source: str = "", target: str = "") -> BackgroundTask:
        with self.lock:
            task = BackgroundTask(self.next_id, cmd, title, source, target, proc=None)
            self.next_id += 1
            self.tasks.append(task)
            return task

    def cancel_task(self, task_id: int) -> bool:
        with self.lock:
            for t in self.tasks:
                if t.id == task_id:
                    if t.status == "running" and t.proc:
                        try:
                            t.proc.terminate()
                        except Exception:
                            pass
                    t.status = "cancelled"
                    t.finished_at = time.time()
                    return True
        return False

    def get_summary(self) -> str:
        with self.lock:
            active = [t for t in self.tasks if t.status == "running"]
            queued = [t for t in self.tasks if t.status == "queued"]
            if not active and not queued:
                return ""
            if active:
                t0 = active[0]
                return f"[⏬ BG: {len(active)+len(queued)} active ({t0.pct:4.1f}%)]"
            return f"[⏬ BG: {len(queued)} queued]"

    def _monitor_proc(self, task: BackgroundTask):
        proc = task.proc
        re_pct = re.compile(r'(\d+(?:\.\d+)?)%')
        while proc.poll() is None:
            try:
                line = proc.stdout.readline()
                if not line:
                    break
                text = line.decode("utf-8", errors="replace").strip()
                if text:
                    task.log_lines.append(text)
                    task.stage = DynamicStageDetector.detect(text, task.stage)
                    m = re_pct.search(text)
                    if m:
                        try:
                            task.pct = float(m.group(1))
                        except ValueError:
                            pass
            except Exception:
                break
        proc.wait()
        task.exit_code = proc.returncode
        task.status = "done" if task.exit_code == 0 else f"failed ({task.exit_code})"
        task.finished_at = time.time()
        add_history_entry(task.title, task.source, task.target, "Success" if task.exit_code == 0 else "Failed")

    def _worker_loop(self):
        while True:
            time.sleep(0.5)
            with self.lock:
                running = [t for t in self.tasks if t.status == "running"]
                queued = [t for t in self.tasks if t.status == "queued"]
                if len(running) < 1 and queued:
                    next_task = queued[0]
                    try:
                        next_task.proc = subprocess.Popen(next_task.cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=0)
                        next_task.status = "running"
                        t = threading.Thread(target=self._monitor_proc, args=(next_task,), daemon=True)
                        t.start()
                    except Exception as e:
                        next_task.status = f"error ({e})"
                        next_task.finished_at = time.time()

BG_QUEUE = BackgroundQueueManager()

# ==========================================================================
# Strings & Localization
# ==========================================================================

STRINGS = {
    "en": {
        "app_title": "MogDop's MediaCLI",
        "tab_online": "Downloads",
        "tab_local": "Local Tools",
        "tab_system": "System & Config",

        "tab_set_gen": "General",
        "tab_set_conv": "Video & Audio",
        "tab_set_ui": "Interface & Theme",
        "tab_set_app": "App & System Config",
        "tab_set_presets": "Manage Presets",
        "tab_set_defaults": "Download Defaults",

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

        "footer_nav": "↑/↓ or j/k - navigate   Enter - select/focus   Esc/q - back   [Alt+Shift+P / F12] - Terminal",
        "footer_input": "Enter - confirm   Esc - cancel   Ctrl+Q - quit",
        "footer_message": "Enter/Esc - continue",
        "footer_vertical_tabs": "↑/↓ - Switch category   Enter/→ - Edit options   Esc - Exit settings",
        "footer_vertical_items": "↑/↓ - Select option   Enter - Change/Toggle   Esc/← - Back to categories",

        "video_title": "Download video",
        "video_prompt_url": "Paste the video URL:",
        "video_quality_subtitle": "Choose max quality",
        "quality_best": "Best available",
        "quality_custom": "Custom...",
        "preset_subtitle": "Choose FFmpeg / Codec Preset",
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
        "batch_no_files": "No media files found in folder.",

        "trim_title": "Trim Media File",
        "trim_prompt_file": "Enter path to video/audio file:",
        "trim_prompt_start": "Start time (HH:MM:SS, e.g. 00:01:30):",
        "trim_prompt_end": "End time (HH:MM:SS, e.g. 00:02:45):",
        "trim_mode_copy": "Fast Stream Copy (-c copy, instant)",
        "trim_mode_reencode": "Re-encode using target preset",

        "probe_title": "Inspect Local File (FFprobe)",
        "probe_prompt_file": "Enter path to media file:",
        "history_title": "Operation History",
        "history_empty": "No operations recorded yet.",

        "custom_ext_prompt": "Target file extension (e.g. mp4, mov, mkv):",
        "custom_flags_prompt": "Custom FFmpeg flags (e.g. -c:v libx264 -c:a aac):",

        "log_title": "Process Execution",
        "log_footer_running": "q/Esc - Move to Background   F10 - Toggle Raw Logs   Ctrl+Q - Terminate",
        "log_footer_done": "Enter/Esc - Return to menu",
        "log_finished_ok": "Done (exit code 0).",
        "log_finished_err": "Finished with exit code {v}.",
        "bg_transferred": "Task transferred to background queue!",

        # Settings
        "settings_title": "Settings & Configuration",
        "settings_download_dir": "Download folder: {v}",
        "settings_goal": "Primary Use-Case: {v}",
        "settings_proxy": "Network Proxy: {v}",
        "settings_language": "Language: {v}",
        "settings_cookies": "Cookies: {v}",
        "settings_preset": "Default Codec Preset: {v}",
        "settings_audio_format": "Default Audio Format: {v}",
        "settings_sub_langs": "Default Subtitle Languages: {v}",
        "settings_theme": "Color Theme: {v}",
        "settings_bg": "Terminal Background: {v}",
        "settings_style": "Progress Bar Style: {v}",
        "settings_bg_queue_max": "Max Background Tasks: {v}",
        "settings_notify_bell": "Terminal Audio Bell on Finish: {v}",
        "settings_auto_check_deps": "Auto-check Dependencies on Launch: {v}",
        "settings_editor": "Default Text Editor ($EDITOR): {v}",
        "settings_reset": "Reset all settings to factory defaults...",

        "download_mode_quick": "Quick Download (use default preset)",
        "download_mode_preset": "Choose a saved preset",
        "download_mode_manual": "Configure manually",
        "download_mode_subtitle": "How would you like to download this?",
        "preset_none_quick": "No default preset set yet. Switching to manual setup.",
        "presets_list_title": "Download Presets",
        "preset_create_new": "+ Create new preset...",
        "preset_default_marker": "[default] ",
        "manual_title": "Manual Download Setup",

        "val_yes": "YES",
        "val_no": "NO",
        "val_default": "(default)",
        "val_custom_ffmpeg": "Custom FFmpeg preset",

        "info_title": "Video Info",
        "info_fetching": "Fetching URL metadata...",
        "subs_title": "Download Subtitles",
        "audio_title": "Download Audio",
        "audio_format_subtitle": "Choose audio target format",
        "playlist_title": "Download Playlist",
        "playlist_prompt_url": "Paste the playlist URL:",
        "update_title": "Update yt-dlp",
        "update_via_pkg": "System Package Manager",
        "update_via_pip": "Python Pip",
        "update_via_ytdlp": "yt-dlp Self-Update (-U)",
        "update_cancel": "Cancel",

        "label_url": "URL:",
        "label_input": "Input File:",
        "label_output": "Output File:",
        "label_preset": "Preset:",
        "label_cmd": "Command:",
        "label_folder": "Folder:",
        "label_format": "Format:",
        "label_langs": "Languages:",

        "proxy_system": "System Proxy (Auto)",
        "proxy_custom": "Custom Proxy",
        "proxy_none": "Direct Connection (No proxy)",

        "wizard_title": "MediaCLI First-Run Setup Wizard",
        "wizard_step1_title": "Step 1/3: Interface Language",
        "wizard_step2_title": "Step 2/3: Primary Use-Case",
        "wizard_step3_title": "Step 3/3: Download Folder",
        "wizard_done_title": "Setup Complete",
        "wizard_done_msg": "Configuration saved! Press Enter to launch MediaCLI.",

        "goal_editing": "Video Editing (DaVinci / Premiere)",
        "goal_downloading": "General Media Downloading",
        "goal_audio": "Audio Extraction & Archiving",
        "goal_transcoding": "FFmpeg Transcoding & Encoding",

        "bg_option_keep": "Transparent / Native Background",
        "bg_option_solid": "Solid Theme Background",
    },
    "ru": {
        "app_title": "MogDop's MediaCLI",
        "tab_online": "Загрузка",
        "tab_local": "Инструменты",
        "tab_system": "Система",

        "tab_set_gen": "Основные",
        "tab_set_conv": "Видео и Аудио",
        "tab_set_ui": "Интерфейс и Тема",
        "tab_set_app": "Настройки программы",
        "tab_set_presets": "Пресеты загрузки",
        "tab_set_defaults": "Настройки по умолчанию",

        "menu_video": "Скачать видео (YouTube/VK/др.)",
        "menu_audio": "Скачать аудиодорожку",
        "menu_playlist": "Скачать плейлист",
        "menu_convert": "Конвертировать локальный файл (FFmpeg)",
        "menu_batch_convert": "Пакетная конвертация папки",
        "menu_trim": "Обрезать видео / аудио",
        "menu_probe": "Анализ медиафайла (FFprobe)",
        "menu_info": "Информация о ссылке",
        "menu_subs": "Скачать субтитры",
        "menu_history": "История операций",
        "menu_cookies": "Cookies",
        "menu_settings": "Настройки",
        "menu_update": "Обновить yt-dlp",
        "menu_exit": "Выход",

        "footer_nav": "↑/↓ или j/k — выбор   Enter — подтвердить   Esc/q — назад   [Alt+Shift+P / F12] — Терминал",
        "footer_input": "Enter — подтвердить   Esc — отмена   Ctrl+Q — выход",
        "footer_message": "Enter/Esc — продолжить",
        "footer_vertical_tabs": "↑/↓ — Выбор категории   Enter/→ — Перейти к параметрам   Esc — Выход",
        "footer_vertical_items": "↑/↓ — Выбор параметра   Enter — Изменить/Вкл   Esc/← — Назад к вкладкам",

        "video_title": "Скачать видео",
        "video_prompt_url": "Введите ссылку на видео:",
        "video_quality_subtitle": "Выберите максимальное качество",
        "quality_best": "Лучшее доступное",
        "quality_custom": "Указать вручную...",
        "preset_subtitle": "Выберите пресет FFmpeg / кодеков",
        "outdir_prompt": "Папка для сохранения:",
        "confirm_title": "Подтверждение",
        "confirm_start": "Начать процесс",
        "confirm_cancel": "Отмена",

        "convert_title": "Конвертировать локальный файл",
        "convert_prompt_file": "Введите полный путь к исходному файлу:",
        "convert_prompt_preset": "Выберите целевой пресет конвертации:",
        "convert_prompt_outdir": "Папка назначения (пусто — та же папка):",
        "convert_err_notfound": "Ошибка: Файл или папка '{f}' не существует!",

        "batch_title": "Пакетная конвертация папки",
        "batch_prompt_folder": "Введите путь к папке с файлами:",
        "batch_no_files": "В выбранной папке не найдено подходящих медиафайлов.",

        "trim_title": "Обрезать медиафайл",
        "trim_prompt_file": "Введите путь к файлу:",
        "trim_prompt_start": "Время начала (ЧЧ:ММ:СС, например 00:01:30):",
        "trim_prompt_end": "Время окончания (ЧЧ:ММ:СС):",
        "trim_mode_copy": "Быстрое копирование (-c copy, без потерь)",
        "trim_mode_reencode": "Перекодировать с выбранным пресетом",

        "probe_title": "Анализ медиафайла (FFprobe)",
        "probe_prompt_file": "Введите путь к медиафайлу:",
        "history_title": "История операций",
        "history_empty": "Записей об операциях пока нет.",

        "custom_ext_prompt": "Расширение итогового файла (например, mp4, mov):",
        "custom_flags_prompt": "Флаги FFmpeg (например, -c:v libx264 -c:a pcm_s16le):",

        "log_title": "Выполнение операции",
        "log_footer_running": "q/Esc — В фоновую очередь   F10 — Полный лог   Ctrl+Q — Отмена",
        "log_footer_done": "Enter/Esc — Назад в меню",
        "log_finished_ok": "Успешно завершено (код 0).",
        "log_finished_err": "Завершено с ошибкой (код {v}).",
        "bg_transferred": "Задача переведена в фоновую очередь!",

        # Settings
        "settings_title": "Настройки программы",
        "settings_download_dir": "Папка загрузок: {v}",
        "settings_goal": "Основная цель применения: {v}",
        "settings_proxy": "Сетевой прокси: {v}",
        "settings_language": "Язык интерфейса: {v}",
        "settings_cookies": "Cookies: {v}",
        "settings_preset": "Пресет видео/FFmpeg: {v}",
        "settings_audio_format": "Формат аудио по умолчанию: {v}",
        "settings_sub_langs": "Языки субтитров по умолчанию: {v}",
        "settings_theme": "Цветовая тема: {v}",
        "settings_bg": "Фон терминала: {v}",
        "settings_style": "Стиль индикатора прогресса: {v}",
        "settings_bg_queue_max": "Макс. фоновых задач в очереди: {v}",
        "settings_notify_bell": "Звуковой сигнал терминала по завершении: {v}",
        "settings_auto_check_deps": "Проверять зависимости при старте: {v}",
        "settings_editor": "Текстовый редактор ($EDITOR): {v}",
        "settings_reset": "Сбросить все настройки к заводским...",

        "download_mode_quick": "Быстрая загрузка (пресет по умолчанию)",
        "download_mode_preset": "Выбрать сохранённый пресет",
        "download_mode_manual": "Настроить параметры вручную",
        "download_mode_subtitle": "Как Вы хотите скачать это видео?",
        "preset_none_quick": "Пресет по умолчанию не задан. Переход к ручной настройке.",
        "presets_list_title": "Пресеты загрузки",
        "preset_create_new": "+ Создать новый пресет...",
        "preset_default_marker": "[по умолчанию] ",
        "manual_title": "Ручная настройка загрузки",

        "val_yes": "ДА",
        "val_no": "НЕТ",
        "val_default": "(по умолчанию)",
        "val_custom_ffmpeg": "Свой пресет FFmpeg",

        "info_title": "Информация о видео",
        "info_fetching": "Запрос метаданных...",
        "subs_title": "Скачать субтитры",
        "audio_title": "Скачать аудио",
        "audio_format_subtitle": "Выберите целевой формат аудио",
        "playlist_title": "Скачать плейлист",
        "playlist_prompt_url": "Введите ссылку на плейлист:",
        "update_title": "Обновить yt-dlp",
        "update_via_pkg": "Через системный пакетный менеджер",
        "update_via_pip": "Через Python Pip",
        "update_via_ytdlp": "Через встроенный yt-dlp -U",
        "update_cancel": "Отмена",

        "label_url": "URL:",
        "label_input": "Исходный файл:",
        "label_output": "Выходной файл:",
        "label_preset": "Пресет:",
        "label_cmd": "Команда:",
        "label_folder": "Папка:",
        "label_format": "Формат:",
        "label_langs": "Языки:",

        "proxy_system": "Системный прокси (Авто)",
        "proxy_custom": "Свой прокси",
        "proxy_none": "Прямое подключение (Без прокси)",

        "wizard_title": "Мастер первоначальной настройки MediaCLI",
        "wizard_step1_title": "Шаг 1/3: Язык интерфейса",
        "wizard_step2_title": "Шаг 2/3: Основное назначение",
        "wizard_step3_title": "Шаг 3/3: Папка для загрузок",
        "wizard_done_title": "Настройка завершена",
        "wizard_done_msg": "Параметры сохранены! Нажмите Enter для запуска.",

        "goal_editing": "Видеомонтаж (DaVinci Resolve / Premiere)",
        "goal_downloading": "Обычная загрузка видео из сети",
        "goal_audio": "Извлечение и архивация аудио",
        "goal_transcoding": "Локальная конвертация FFmpeg",

        "bg_option_keep": "Прозрачный / Системный фон",
        "bg_option_solid": "Сплошной темный фон темы",
    }
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

# ==========================================================================
# Config Loader & Cookie Handlers
# ==========================================================================

def load_config() -> dict:
    cfg = dict(DEFAULT_CONFIG)
    if CONFIG_PATH.exists():
        try:
            cfg.update(json.loads(CONFIG_PATH.read_text(encoding="utf-8")))
        except Exception:
            pass
    return cfg

def save_config(cfg: dict) -> None:
    CONFIG_PATH.parent.mkdir(parents=True, exist_ok=True)
    CONFIG_PATH.write_text(json.dumps(cfg, indent=2, ensure_ascii=False), encoding="utf-8")

def cookie_args(cfg: dict) -> list[str]:
    mode = cfg.get("cookies_mode", "none")
    if mode == "file" and cfg.get("cookies_file"):
        cpath = parse_user_path(cfg["cookies_file"])
        if cpath.exists():
            return ["--cookies", str(cpath)]
    if mode == "browser" and cfg.get("cookies_browser"):
        return ["--cookies-from-browser", cfg["cookies_browser"]]
    return []

def parse_cookies_file(path: str) -> list[dict]:
    p = parse_user_path(path)
    if not p.exists():
        return []
    cookies = []
    for line in p.read_text(errors="replace").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split("\t")
        if len(parts) == 7:
            cookies.append({"domain": parts[0], "name": parts[5], "value": parts[6]})
    return cookies

# ==========================================================================
# Schema with CLI Argument Annotations (Requirement 4)
# ==========================================================================

DOWNLOAD_PRESET_CATEGORIES = [
    {"title_key": "tab_set_gen", "fields": [
        {"key": "quality", "label": "Max Quality (height in px, e.g. 1080)", "cli": "-f bestvideo[height<=?]", "type": "text"},
        {"key": "output_template", "label": "Filename Output Template", "cli": "-o template", "type": "text"},
        {"key": "restrict_filenames", "label": "Restrict Filenames (ASCII only, no spaces)", "cli": "--restrict-filenames", "type": "bool"},
        {"key": "retries", "label": "Connection Retries", "cli": "--retries N", "type": "text"},
        {"key": "concurrent_fragments", "label": "Concurrent Fragment Downloads", "cli": "--concurrent-fragments N", "type": "choice", "choices": [("", "(default)"), ("4", "4"), ("8", "8"), ("16", "16")]},
    ]},
    {"title_key": "tab_set_conv", "fields": [
        {"key": "video_preset", "label": "Video Codec Export Preset", "cli": "--recode-video / --postprocessor-args", "type": "video_preset"},
        {"key": "vcodec", "label": "Video Codec Preference", "cli": "-f bestvideo[vcodec^=?]", "type": "choice", "choices": [("auto", "Auto"), ("av1", "Force AV1"), ("vp9", "Force VP9"), ("h264", "Force H.264")]},
        {"key": "fps_limit", "label": "FPS Limit", "cli": "-f bestvideo[fps<=?]", "type": "choice", "choices": [("", "Max"), ("60", "60 FPS"), ("30", "30 FPS")]},
        {"key": "audio_only", "label": "Extract Audio Only", "cli": "-x / --extract-audio", "type": "bool"},
        {"key": "audio_format", "label": "Audio Format", "cli": "--audio-format fmt", "type": "choice", "choices": [(a, a.upper()) for a in AUDIO_FORMATS]},
        {"key": "audio_quality", "label": "Audio Quality", "cli": "--audio-quality Q", "type": "choice", "choices": [("0", "Best (0)"), ("2", "High (2)"), ("5", "Medium (5)"), ("9", "Smallest (9)")]},
    ]},
    {"title_key": "cat_subtitles", "fields": [
        {"key": "subs_enabled", "label": "Download Subtitles", "cli": "--write-subs", "type": "bool"},
        {"key": "sub_langs", "label": "Subtitle Languages", "cli": "--sub-langs langs", "type": "text"},
        {"key": "auto_subs", "label": "Include Auto-Generated Subtitles", "cli": "--write-auto-subs", "type": "bool"},
        {"key": "embed_subs", "label": "Embed Subtitles into Video Container", "cli": "--embed-subs", "type": "bool"},
    ]},
    {"title_key": "cat_thumbmeta", "fields": [
        {"key": "embed_metadata", "label": "Embed Metadata & Cover Art", "cli": "--embed-metadata --embed-thumbnail", "type": "bool"},
        {"key": "embed_chapters", "label": "Embed Chapters", "cli": "--embed-chapters", "type": "bool"},
        {"key": "write_extra", "label": "Save Description & Thumbnail as Files", "cli": "--write-description --write-thumbnail", "type": "bool"},
        {"key": "sponsorblock", "label": "SponsorBlock (Cut Sponsorships)", "cli": "--sponsorblock-remove categories", "type": "choice", "choices": [("off", "Disabled"), ("sponsors", "Sponsors Only"), ("sponsors_promo", "Sponsors + Intros + Promos")]},
    ]},
    {"title_key": "tab_system", "fields": [
        {"key": "proxy_mode", "label": "Proxy Mode", "cli": "--proxy url", "type": "proxy_mode"},
        {"key": "ratelimit", "label": "Speed Rate Limit", "cli": "--limit-rate rate", "type": "choice", "choices": [("", "Unlimited"), ("10M", "10 MB/s"), ("5M", "5 MB/s"), ("2M", "2 MB/s")]},
        {"key": "geobypass", "label": "Bypass Geographic Restrictions", "cli": "--geo-bypass", "type": "bool"},
        {"key": "live_start", "label": "Download Live Streams from Start", "cli": "--live-from-start", "type": "bool"},
    ]}
]

def _builtin_preset_defaults(cfg: dict) -> dict:
    return {
        "quality": "",
        "video_preset": cfg.get("video_preset", "davinci_dnxhr"),
        "custom_ext": "mp4",
        "custom_flags": "-c:v libx264 -c:a aac",
        "vcodec": "auto",
        "fps_limit": "",
        "audio_only": False,
        "audio_format": cfg.get("audio_format", "mp3"),
        "audio_quality": "0",
        "subs_enabled": False,
        "sub_langs": cfg.get("sub_langs", "ru,en"),
        "auto_subs": False,
        "embed_subs": False,
        "embed_metadata": True,
        "embed_chapters": True,
        "write_extra": False,
        "sponsorblock": "off",
        "proxy_mode": cfg.get("proxy_mode", "system"),
        "proxy_url": cfg.get("proxy_url", ""),
        "ratelimit": "",
        "geobypass": False,
        "live_start": False,
        "concurrent_fragments": "",
        "retries": "",
        "restrict_filenames": False,
        "output_template": "",
    }

def get_effective_default_fields(cfg: dict) -> dict:
    fields = _builtin_preset_defaults(cfg)
    fields.update(cfg.get("preset_defaults") or {})
    return fields

def build_ytdlp_args_from_preset(preset: dict, cfg: dict, out_dir: Path, for_playlist: bool = False) -> list[str]:
    f = preset.get("fields", {})
    cmd: list[str] = []

    if f.get("restrict_filenames"):
        cmd += ["--restrict-filenames"]
    if f.get("retries"):
        cmd += ["--retries", str(f["retries"])]
    if f.get("concurrent_fragments"):
        cmd += ["--concurrent-fragments", str(f["concurrent_fragments"])]

    if f.get("audio_only"):
        cmd += ["-x", "--audio-format", f.get("audio_format", "mp3"), "--audio-quality", str(f.get("audio_quality", "0"))]
    else:
        quality = f.get("quality", "")
        fps_suffix = f"[fps<={f['fps_limit']}]" if f.get("fps_limit") else ""
        q_str = f"[height<={quality}]" if quality else ""
        vc_filter = {"av1": "[vcodec^=av01]", "vp9": "[vcodec^=vp9]", "h264": "[vcodec^=avc1]"}.get(f.get("vcodec", "auto"), "")
        fmt = f"bestvideo{q_str}{fps_suffix}{vc_filter}+bestaudio/best{q_str}{fps_suffix}"
        cmd += ["-f", fmt]

        v_preset = f.get("video_preset", "davinci_dnxhr")
        if v_preset == "custom":
            ext = (f.get("custom_ext") or "mp4").strip(".")
            flags = f.get("custom_flags") or "-c:v libx264 -c:a aac"
            cmd += ["--recode-video", ext, "--postprocessor-args", f"ffmpeg:{flags}"]
        elif v_preset in VIDEO_PRESETS:
            cmd += VIDEO_PRESETS[v_preset]["args"]

    if f.get("geobypass"):
        cmd += ["--geo-bypass"]

    # Fixed proxy handling: append proxy arguments ONLY once
    cmd += get_proxy_args(cfg, override_mode=f.get("proxy_mode"), override_url=f.get("proxy_url"))

    if f.get("ratelimit"):
        cmd += ["--limit-rate", str(f["ratelimit"])]
    if f.get("live_start"):
        cmd += ["--live-from-start"]
    if f.get("write_extra"):
        cmd += ["--write-description", "--write-thumbnail"]

    sb = f.get("sponsorblock", "off")
    if sb != "off":
        cats = "sponsor" if sb == "sponsors" else "sponsor,selfpromo,interaction"
        cmd += ["--sponsorblock-remove", cats]

    if not f.get("audio_only"):
        if f.get("embed_metadata", True):
            cmd += ["--embed-metadata", "--embed-thumbnail"]
        if f.get("embed_chapters", True):
            cmd += ["--embed-chapters"]

    if f.get("subs_enabled"):
        cmd += ["--write-subs", "--sub-langs", f.get("sub_langs", "ru,en")]
        if f.get("auto_subs"):
            cmd += ["--write-auto-subs"]
        if f.get("embed_subs"):
            cmd += ["--embed-subs"]

    template = f.get("output_template") or ("%(playlist_title)s/%(playlist_index)03d - %(title)s.%(ext)s" if for_playlist else "%(title)s.%(ext)s")
    cmd += ["-o", str(out_dir / template)]
    cmd += cookie_args(cfg)
    return cmd

# ==========================================================================
# Curses Drawing & Overlay Widgets
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
    safe_addstr(stdscr, h - 1, 2, text, w - 24, curses.A_DIM)
    bg_sum = BG_QUEUE.get_summary()
    if bg_sum:
        safe_addstr(stdscr, h - 1, max(2, w - len(bg_sum) - 2), bg_sum, len(bg_sum) + 2, curses.color_pair(3) | curses.A_BOLD if curses else curses.A_BOLD)

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
    else:
        arrow = ">" if filled < inner_w else ""
        spaces = " " * max(0, unfilled - len(arrow))
        bar = "=" * filled + arrow + spaces
    return f"[{bar}]"

# ==========================================================================
# Embedded Overlay Terminal (Requirement 7)
# ==========================================================================

class EmbeddedTerminal:
    """A floating terminal overlay accessible via Alt+Shift+P or F12 from any screen."""

    def __init__(self):
        self.history_lines: list[str] = [
            "MediaCLI Interactive Console. Type 'help' or '?' for commands.",
            "Press Alt+Shift+P, F12, or Esc to minimize. Type 'exit' to close.",
            ""
        ]
        self.cmd_history: list[str] = []
        self.cmd_idx: int = 0
        self.current_buf: list[str] = []

    def open(self, stdscr, cfg: dict):
        curses.curs_set(1)
        while True:
            h, w = stdscr.getmaxyx()
            stdscr.erase()

            # Draw outer box
            for y in range(h):
                safe_addstr(stdscr, y, 0, "|" if 0 < y < h - 1 else "+", 1, curses.A_DIM)
                safe_addstr(stdscr, y, w - 1, "|" if 0 < y < h - 1 else "+", 1, curses.A_DIM)
            safe_addstr(stdscr, 0, 0, "+" + "=" * (w - 2) + "+", w, curses.A_BOLD)
            safe_addstr(stdscr, h - 1, 0, "+" + "=" * (w - 2) + "+", w, curses.A_BOLD)

            title = " [ MediaCLI Terminal Overlay — Alt+Shift+P / F12 / Esc to minimize ] "
            safe_addstr(stdscr, 0, max(2, (w - len(title)) // 2), title, w - 4, curses.color_pair(4) | curses.A_BOLD if curses else curses.A_BOLD)

            # Output lines
            max_log_rows = h - 4
            visible_lines = self.history_lines[-max_log_rows:]
            for idx, line in enumerate(visible_lines):
                safe_addstr(stdscr, 1 + idx, 2, line, w - 4)

            # Prompt line
            prompt = "mediacli> "
            safe_addstr(stdscr, h - 2, 2, "-" * (w - 4), w - 4, curses.A_DIM)
            safe_addstr(stdscr, h - 2, 2, prompt, len(prompt), curses.color_pair(5) | curses.A_BOLD if curses else curses.A_BOLD)
            input_text = "".join(self.current_buf)
            safe_addstr(stdscr, h - 2, 2 + len(prompt), input_text, w - len(prompt) - 4)

            try:
                stdscr.move(h - 2, min(w - 2, 2 + len(prompt) + len(self.current_buf)))
            except curses.error:
                pass
            stdscr.refresh()

            key = stdscr.getch()

            if key in (curses.KEY_F12, 276):
                curses.curs_set(0)
                return
            if key == 27:  # Esc or Alt sequence
                stdscr.nodelay(True)
                next_k = stdscr.getch()
                stdscr.nodelay(False)
                if next_k in (ord('P'), ord('p'), 80, 112, -1):
                    curses.curs_set(0)
                    return
                if next_k != -1:
                    curses.ungetch(next_k)
                curses.curs_set(0)
                return

            if key in (curses.KEY_ENTER, 10, 13):
                cmd_line = "".join(self.current_buf).strip()
                self.current_buf = []
                if not cmd_line:
                    continue
                self.cmd_history.append(cmd_line)
                self.cmd_idx = len(self.cmd_history)
                self.history_lines.append(f"mediacli> {cmd_line}")

                if cmd_line.lower() in ("exit", "quit", "close"):
                    curses.curs_set(0)
                    return
                elif cmd_line.lower() in ("clear", "cls"):
                    self.history_lines = []
                else:
                    self._dispatch_command(cmd_line, cfg)

            elif key in (curses.KEY_UP, ord('k')):
                if self.cmd_history and self.cmd_idx > 0:
                    self.cmd_idx -= 1
                    self.current_buf = list(self.cmd_history[self.cmd_idx])
            elif key in (curses.KEY_DOWN, ord('j')):
                if self.cmd_history and self.cmd_idx < len(self.cmd_history) - 1:
                    self.cmd_idx += 1
                    self.current_buf = list(self.cmd_history[self.cmd_idx])
                else:
                    self.cmd_idx = len(self.cmd_history)
                    self.current_buf = []
            elif key in (curses.KEY_BACKSPACE, 127, 8):
                if self.current_buf:
                    self.current_buf.pop()
            elif 32 <= key <= 126:
                self.current_buf.append(chr(key))

    def _dispatch_command(self, cmd_line: str, cfg: dict):
        parts = cmd_line.split()
        if not parts:
            return
        cmd = parts[0].lower()
        args = parts[1:]

        if cmd in ("help", "?"):
            self.history_lines += [
                "Available Terminal Commands:",
                "  config list / get <k> / set <k> <v> - Inspect and modify settings",
                "  preset list / show <id> / delete <id> - Manage download presets",
                "  queue [list|cancel <id>]            - View/cancel background tasks",
                "  history [list|clear]               - View or wipe operation history",
                "  cookies [list|add <dom> <k> <v>]   - Manage cookies.txt file",
                "  proxy [status|set <mode> [url]]    - Change active network proxy",
                "  theme <name>                       - Switch color theme directly",
                "  doctor                             - Diagnose dependencies & tools",
                "  dl <url> [-q 1080] [-p preset]     - Queue download to background",
                "  convert <file> [-p preset]         - Convert local media file",
                "  probe <file>                       - Print stream details with ffprobe",
                "  clear / cls                        - Clear console buffer",
                "  exit / quit                        - Close terminal overlay"
            ]

        elif cmd == "config":
            if not args or args[0] == "list":
                for k, v in sorted(cfg.items()):
                    if not isinstance(v, (list, dict)):
                        self.history_lines.append(f"  {k} = {v}")
            elif args[0] == "get" and len(args) > 1:
                self.history_lines.append(f"  {args[1]} = {cfg.get(args[1], '<not set>')}")
            elif args[0] == "set" and len(args) > 2:
                key = args[1]
                val = " ".join(args[2:])
                if val.lower() == "true": val = True
                elif val.lower() == "false": val = False
                elif val.isdigit(): val = int(val)
                cfg[key] = val
                save_config(cfg)
                apply_theme(cfg)
                self.history_lines.append(f"  [✓] Set {key} = {val}")
            else:
                self.history_lines.append("  Usage: config [list | get <key> | set <key> <value>]")

        elif cmd == "preset":
            presets = cfg.get("download_presets", [])
            if not args or args[0] == "list":
                self.history_lines.append(f"Saved Presets ({len(presets)}):")
                for p in presets:
                    def_marker = " [DEFAULT]" if p["id"] == cfg.get("default_download_preset") else ""
                    self.history_lines.append(f"  - {p['id']}: {p['name']}{def_marker}")
            elif args[0] == "show" and len(args) > 1:
                p = next((p for p in presets if p["id"] == args[1] or p["name"] == args[1]), None)
                if p:
                    self.history_lines.append(f"Preset {p['name']} ({p['id']}):")
                    for fk, fv in p.get("fields", {}).items():
                        self.history_lines.append(f"    {fk}: {fv}")
                else:
                    self.history_lines.append(f"  [!] Preset '{args[1]}' not found.")
            elif args[0] == "delete" and len(args) > 1:
                cfg["download_presets"] = [p for p in presets if p["id"] != args[1] and p["name"] != args[1]]
                save_config(cfg)
                self.history_lines.append(f"  [✓] Deleted preset {args[1]}")
            else:
                self.history_lines.append("  Usage: preset [list | show <id> | delete <id>]")

        elif cmd in ("queue", "bg"):
            if not args or args[0] == "list":
                if not BG_QUEUE.tasks:
                    self.history_lines.append("  Background queue is empty.")
                for t in BG_QUEUE.tasks:
                    self.history_lines.append(f"  [{t.id}] {t.title} | Status: {t.status} | Stage: {t.stage} ({t.pct:.1f}%)")
            elif args[0] == "cancel" and len(args) > 1:
                tid = int(args[1]) if args[1].isdigit() else -1
                if BG_QUEUE.cancel_task(tid):
                    self.history_lines.append(f"  [✓] Cancelled background task #{tid}")
                else:
                    self.history_lines.append(f"  [!] Task #{tid} not found.")

        elif cmd == "history":
            if not args or args[0] == "list":
                if not HISTORY_PATH.exists():
                    self.history_lines.append("  No history entries.")
                else:
                    entries = json.loads(HISTORY_PATH.read_text(encoding="utf-8"))[:10]
                    for e in entries:
                        self.history_lines.append(f"  [{e.get('time')}] {e.get('type')} ({e.get('status')}): {e.get('source')}")
            elif args[0] == "clear":
                HISTORY_PATH.write_text("[]", encoding="utf-8")
                self.history_lines.append("  [✓] Operation history cleared.")

        elif cmd == "cookies":
            cfile = cfg.get("cookies_file") or DEFAULT_COOKIES_FILE
            if not args or args[0] == "list":
                cookies = parse_cookies_file(cfile)
                self.history_lines.append(f"Cookies in {cfile} ({len(cookies)}):")
                for c in cookies[:8]:
                    self.history_lines.append(f"  {c['domain']} -> {c['name']}")
            elif args[0] == "add" and len(args) >= 4:
                domain, name, val = args[1], args[2], args[3]
                p = parse_user_path(cfile)
                p.parent.mkdir(parents=True, exist_ok=True)
                exp = str(int(time.time()) + 3600*24*365)
                flag = "TRUE" if domain.startswith(".") else "FALSE"
                with p.open("a", encoding="utf-8") as f:
                    f.write(f"{domain}\t{flag}\t/\tFALSE\t{exp}\t{name}\t{val}\n")
                self.history_lines.append(f"  [✓] Cookie added: {domain} {name}")

        elif cmd == "theme":
            if args and args[0] in THEMES:
                cfg["theme"] = args[0]
                save_config(cfg)
                apply_theme(cfg)
                self.history_lines.append(f"  [✓] Theme changed to {args[0]}")
            else:
                self.history_lines.append(f"  Available themes: {', '.join(THEMES.keys())}")

        elif cmd == "doctor":
            bins = ["yt-dlp", "ffmpeg", "ffprobe", "atomicparsley", "deno"]
            self.history_lines.append("System Diagnostics:")
            for b in bins:
                path = shutil.which(b)
                status = f"FOUND ({path})" if path else "MISSING"
                self.history_lines.append(f"  - {b:15s}: {status}")

        elif cmd in ("dl", "download") and args:
            url = args[0]
            out_dir = parse_user_path(cfg["download_dir"])
            fields = get_effective_default_fields(cfg)
            preset_obj = {"id": "cli", "name": "Terminal DL", "fields": fields}
            cmd_list = ["yt-dlp", *build_ytdlp_args_from_preset(preset_obj, cfg, out_dir), url]
            task = BG_QUEUE.enqueue_cmd(cmd_list, "Download Video", source=url, target=str(out_dir))
            self.history_lines.append(f"  [✓] Download enqueued to background as Task #{task.id}")

        else:
            self.history_lines.append(f"  [!] Unknown command: '{cmd}'. Type 'help' for instructions.")

TERMINAL_OVERLAY = EmbeddedTerminal()

def check_terminal_hotkey(stdscr, key: int) -> bool:
    if key in (curses.KEY_F12, 276):
        return True
    if key == 27:  # Esc or Alt sequence
        stdscr.nodelay(True)
        next_k = stdscr.getch()
        stdscr.nodelay(False)
        if next_k in (ord('P'), ord('p'), 80, 112):
            return True
        if next_k != -1:
            curses.ungetch(next_k)
    return False

# ==========================================================================
# Vertical Split Settings Screen (Requirement 2 & 3 & 4)
# ==========================================================================

def screen_settings_vertical(stdscr, cfg: dict) -> None:
    """Vertical split settings with Categories on Left and Options on Right."""
    left_idx = 0
    right_idx = 0
    in_right_pane = False
    curses.curs_set(0)

    categories = [
        {"id": "gen", "title": t(cfg, "tab_set_gen")},
        {"id": "conv", "title": t(cfg, "tab_set_conv")},
        {"id": "ui", "title": t(cfg, "tab_set_ui")},
        {"id": "app", "title": t(cfg, "tab_set_app")},
        {"id": "presets", "title": t(cfg, "tab_set_presets")},
        {"id": "defaults", "title": t(cfg, "tab_set_defaults")},
    ]

    while True:
        stdscr.erase()
        h, w = stdscr.getmaxyx()
        draw_header(stdscr, t(cfg, "settings_title"), w)

        left_w = 26
        divider_x = left_w + 2

        # Draw left categories
        for i, cat in enumerate(categories):
            y = 3 + i
            is_sel = (i == left_idx)
            attr = HL_ATTR if (is_sel and not in_right_pane) else (curses.A_BOLD if is_sel else curses.A_NORMAL)
            marker = "► " if is_sel else "  "
            safe_addstr(stdscr, y, 2, f"{marker}{cat['title']}", left_w, attr)

        # Draw vertical separator
        for y in range(2, h - 2):
            safe_addstr(stdscr, y, divider_x, "|", 1, curses.A_DIM)

        # Generate current right items
        active_cat = categories[left_idx]["id"]
        right_items: list[tuple[str, str, str]] = []  # (Label, CLI Flag Hint, Key)

        if active_cat == "gen":
            right_items = [
                (t(cfg, "settings_download_dir", v=cfg["download_dir"]), "-o / --output", "download_dir"),
                (t(cfg, "settings_goal", v=cfg.get("user_goal", "editing")), "workflow optimization", "user_goal"),
                (t(cfg, "settings_proxy", v=(cfg.get("proxy_url") if cfg.get("proxy_mode") == "custom" else t(cfg, f"proxy_{cfg.get('proxy_mode', 'system')}"))), "--proxy", "proxy_mode"),
                (t(cfg, "settings_language", v="English" if cfg.get("language") == "en" else "Русский"), "i18n locale", "language"),
                (t(cfg, "settings_cookies", v=cfg.get("cookies_mode", "none")), "--cookies / --cookies-from-browser", "cookies"),
            ]
        elif active_cat == "conv":
            right_items = [
                (t(cfg, "settings_preset", v=get_preset_name(cfg, cfg.get("video_preset", "davinci_dnxhr"))), "--recode-video / --postprocessor-args", "video_preset"),
                (t(cfg, "settings_audio_format", v=cfg["audio_format"]), "--audio-format", "audio_format"),
                (t(cfg, "settings_sub_langs", v=cfg["sub_langs"]), "--sub-langs", "sub_langs"),
            ]
        elif active_cat == "ui":
            theme_name = THEMES.get(cfg.get("theme", "cyan"), THEMES["cyan"])["name_ru" if cfg.get("language")=="ru" else "name_en"]
            right_items = [
                (t(cfg, "settings_theme", v=theme_name), "curses color pair", "theme"),
                (t(cfg, "settings_bg", v=t(cfg, "bg_option_keep" if cfg.get("use_terminal_bg", True) else "bg_option_solid")), "use_default_colors()", "use_terminal_bg"),
                (t(cfg, "settings_style", v=cfg.get("progress_style", "blocks")), "ASCII/Unicode style", "progress_style"),
            ]
        elif active_cat == "app":
            right_items = [
                (t(cfg, "settings_bg_queue_max", v=cfg.get("bg_queue_max", 3)), "FIFO Queue slots", "bg_queue_max"),
                (t(cfg, "settings_notify_bell", v="YES" if cfg.get("notify_bell", True) else "NO"), "\\a terminal bell", "notify_bell"),
                (t(cfg, "settings_auto_check_deps", v="YES" if cfg.get("auto_check_deps", True) else "NO"), "startup pacman check", "auto_check_deps"),
                (t(cfg, "settings_editor", v=cfg.get("default_editor") or os.environ.get("EDITOR", "nano")), "$EDITOR path", "editor"),
                (t(cfg, "settings_reset"), "factory wipe", "reset"),
            ]
        elif active_cat == "presets":
            right_items = [(f"Manage Presets ({len(cfg.get('download_presets', []))} saved)", "CRUD manager", "open_presets")]
        elif active_cat == "defaults":
            right_items = [(t(cfg, "tab_set_defaults"), "Global download template", "open_defaults")]

        # Render right items
        right_x = divider_x + 3
        right_w = w - right_x - 2
        right_idx = min(right_idx, max(0, len(right_items) - 1))

        for idx, (label, cli_hint, _) in enumerate(right_items):
            y = 3 + idx
            if y >= h - 4:
                break
            is_item_sel = (idx == right_idx and in_right_pane)
            attr = HL_ATTR if is_item_sel else curses.A_NORMAL
            marker = "> " if is_item_sel else "  "

            # Render item with dimmed CLI argument hint
            safe_addstr(stdscr, y, right_x, f"{marker}{label}", right_w - len(cli_hint) - 3, attr)
            safe_addstr(stdscr, y, w - len(cli_hint) - 4, f"[{cli_hint}]", len(cli_hint) + 2, curses.A_DIM)

        footer_msg = t(cfg, "footer_vertical_items" if in_right_pane else "footer_vertical_tabs")
        draw_footer(stdscr, footer_msg, w, h)
        stdscr.refresh()

        key = stdscr.getch()

        if check_terminal_hotkey(stdscr, key):
            TERMINAL_OVERLAY.open(stdscr, cfg)
            continue

        if not in_right_pane:
            if key in (curses.KEY_UP, ord('k')):
                left_idx = (left_idx - 1) % len(categories)
                right_idx = 0
            elif key in (curses.KEY_DOWN, ord('j')):
                left_idx = (left_idx + 1) % len(categories)
                right_idx = 0
            elif key in (curses.KEY_ENTER, 10, 13, curses.KEY_RIGHT, ord('l')):
                in_right_pane = True
                right_idx = 0
            elif key in (27, ord('q')):
                return
        else:
            if key in (curses.KEY_UP, ord('k')):
                right_idx = (right_idx - 1) % len(right_items) if right_items else 0
            elif key in (curses.KEY_DOWN, ord('j')):
                right_idx = (right_idx + 1) % len(right_items) if right_items else 0
            elif key in (27, curses.KEY_LEFT, ord('h')):
                in_right_pane = False
            elif key in (curses.KEY_ENTER, 10, 13):
                if right_items:
                    target_key = right_items[right_idx][2]
                    _handle_setting_edit(stdscr, cfg, target_key)

def _handle_setting_edit(stdscr, cfg: dict, key: str):
    if key == "download_dir":
        val = text_input(stdscr, t(cfg, "settings_title"), "New Download Directory:", t(cfg, "footer_input"), default=cfg["download_dir"])
        if val: cfg["download_dir"] = val; save_config(cfg)
    elif key == "user_goal":
        goals = ["editing", "downloading", "audio", "transcoding"]
        labels = [t(cfg, f"goal_{g}") for g in goals]
        gi = run_menu(stdscr, t(cfg, "settings_title"), labels, t(cfg, "footer_nav"))
        if gi is not None: cfg["user_goal"] = goals[gi]; save_config(cfg)
    elif key == "proxy_mode":
        modes = ["system", "custom", "none"]
        pi = run_menu(stdscr, t(cfg, "settings_title"), [t(cfg, f"proxy_{m}") for m in modes], t(cfg, "footer_nav"))
        if pi is not None:
            cfg["proxy_mode"] = modes[pi]
            if modes[pi] == "custom":
                pr = text_input(stdscr, t(cfg, "settings_title"), "Proxy URL (e.g. socks5://127.0.0.1:10808):", t(cfg, "footer_input"), default=cfg.get("proxy_url", ""))
                if pr is not None: cfg["proxy_url"] = pr.strip()
            save_config(cfg)
    elif key == "language":
        cfg["language"] = "ru" if cfg.get("language") == "en" else "en"
        save_config(cfg)
    elif key == "cookies":
        screen_cookies(stdscr, cfg)
    elif key == "video_preset":
        keys = get_ordered_video_preset_keys(cfg)
        labels = [VIDEO_PRESETS[k]["name_ru" if cfg.get("language")=="ru" else "name_en"] for k in keys]
        pi = run_menu(stdscr, t(cfg, "settings_title"), labels, t(cfg, "footer_nav"))
        if pi is not None: cfg["video_preset"] = keys[pi]; save_config(cfg)
    elif key == "audio_format":
        fi = run_menu(stdscr, t(cfg, "settings_title"), [a.upper() for a in AUDIO_FORMATS], t(cfg, "footer_nav"))
        if fi is not None: cfg["audio_format"] = AUDIO_FORMATS[fi]; save_config(cfg)
    elif key == "sub_langs":
        val = text_input(stdscr, t(cfg, "settings_title"), "Subtitle languages (comma separated):", t(cfg, "footer_input"), default=cfg["sub_langs"])
        if val is not None: cfg["sub_langs"] = val.strip(); save_config(cfg)
    elif key == "theme":
        t_keys = list(THEMES.keys())
        labels = [THEMES[k]["name_ru" if cfg.get("language")=="ru" else "name_en"] for k in t_keys]
        ti = run_menu(stdscr, t(cfg, "settings_title"), labels, t(cfg, "footer_nav"))
        if ti is not None: cfg["theme"] = t_keys[ti]; apply_theme(cfg); save_config(cfg)
    elif key == "use_terminal_bg":
        cfg["use_terminal_bg"] = not cfg.get("use_terminal_bg", True)
        apply_theme(cfg); save_config(cfg)
    elif key == "progress_style":
        styles = ["blocks", "classic", "dots", "minimal"]
        si = run_menu(stdscr, t(cfg, "settings_title"), styles, t(cfg, "footer_nav"))
        if si is not None: cfg["progress_style"] = styles[si]; save_config(cfg)
    elif key == "notify_bell":
        cfg["notify_bell"] = not cfg.get("notify_bell", True); save_config(cfg)
    elif key == "auto_check_deps":
        cfg["auto_check_deps"] = not cfg.get("auto_check_deps", True); save_config(cfg)
    elif key == "bg_queue_max":
        val = text_input(stdscr, t(cfg, "settings_title"), "Max parallel background queue size (1-5):", t(cfg, "footer_input"), default=str(cfg.get("bg_queue_max", 3)))
        if val and val.isdigit(): cfg["bg_queue_max"] = int(val); save_config(cfg)
    elif key == "open_presets":
        screen_manage_download_presets(stdscr, cfg)
    elif key == "open_defaults":
        fields = get_effective_default_fields(cfg)
        result = screen_manual_preset_config(stdscr, cfg, fields)
        if result and result[0] == "save":
            cfg["preset_defaults"] = result[1]
            save_config(cfg)
    elif key == "reset":
        confirm = run_menu(stdscr, t(cfg, "confirm_title"), [t(cfg, "confirm_start"), t(cfg, "confirm_cancel")], t(cfg, "footer_nav"), subtitle="Factory reset configuration?")
        if confirm == 0:
            cfg.clear()
            cfg.update(DEFAULT_CONFIG)
            save_config(cfg)

# ==========================================================================
# Core Foreground Process Runner with Live Single-Line Dynamic Stage
# ==========================================================================

def run_with_log(stdscr, cfg: dict, cmd: list[str], op_type: str = "Task", source: str = "", target: str = "") -> None:
    curses.curs_set(0)
    style = cfg.get("progress_style", "blocks")

    try:
        proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=0)
    except FileNotFoundError:
        show_message(stdscr, t(cfg, "log_title"), ["Executable not found. Please install dependencies."], t(cfg, "footer_message"))
        return

    stdscr.nodelay(True)
    lines: list[str] = [f"[cmd] {' '.join(cmd)}", ""]
    current_stage = "Initializing process..."
    pct = 0.0
    speed_str = ""
    show_logs = False
    buf = ""
    re_pct = re.compile(r'(\d+(?:\.\d+)?)%')
    re_speed = re.compile(r'(?:at|speed=)\s*([^\s]+)')

    while proc.poll() is None:
        try:
            key = stdscr.getch()
        except curses.error:
            key = -1

        if check_terminal_hotkey(stdscr, key):
            TERMINAL_OVERLAY.open(stdscr, cfg)
            continue

        if key in (getattr(curses, "KEY_F10", 274), 274):
            show_logs = not show_logs

        # Requirement 6: Press 'q' or 'Esc' to transfer process to background queue
        if key in (ord('q'), 27):
            BG_QUEUE.add_running_process(proc, cmd, op_type, source, target, lines)
            stdscr.nodelay(False)
            show_message(stdscr, t(cfg, "log_title"), [t(cfg, "bg_transferred")], t(cfg, "footer_message"))
            return

        if key == 17:  # Ctrl+Q terminate
            proc.terminate()
            break

        readable, _, _ = select.select([proc.stdout], [], [], 0.05)
        if readable:
            chunk = proc.stdout.read(4096)
            if chunk:
                text_chunk = chunk.decode("utf-8", errors="replace")
                buf += text_chunk
                for line in text_chunk.splitlines():
                    current_stage = DynamicStageDetector.detect(line, current_stage)
                    m = re_pct.search(line)
                    if m:
                        try: pct = float(m.group(1))
                        except ValueError: pass
                    ms = re_speed.search(line)
                    if ms: speed_str = ms.group(1)
                while "\n" in buf:
                    p, buf = buf.split("\n", 1)
                    if p.strip(): lines.append(p.strip())

        h, w = stdscr.getmaxyx()
        stdscr.erase()
        draw_header(stdscr, t(cfg, "log_title"), w)

        if show_logs:
            log_h = max(0, h - 4)
            for idx, line in enumerate(lines[-log_h:]):
                safe_addstr(stdscr, 3 + idx, 2, line, w - 4)
        else:
            # Single-line prominent stage at TOP (Requirement 5)
            safe_addstr(stdscr, 3, 4, f"Active Stage: {current_stage}", w - 8, curses.color_pair(4) | curses.A_BOLD if curses else curses.A_BOLD)
            safe_addstr(stdscr, 4, 4, "-" * (w - 8), w - 8, curses.A_DIM)

            safe_addstr(stdscr, 6, 4, f"Operation : {op_type}", w - 8)
            if source: safe_addstr(stdscr, 7, 4, f"Source    : {source}", w - 8, curses.A_DIM)

            bar_str = render_progress_bar(pct, min(44, max(20, w - 24)), style)
            safe_addstr(stdscr, 9, 4, f"Progress  : {bar_str} {pct:5.1f}% {('(' + speed_str + ')') if speed_str else ''}", w - 8, curses.A_BOLD)

            safe_addstr(stdscr, 12, 4, "[Press F10 for raw logs | 'q' to move to background queue]", w - 8, curses.A_DIM)

        draw_footer(stdscr, t(cfg, "log_footer_running"), w, h)
        stdscr.refresh()

    proc.wait()
    stdscr.nodelay(False)
    rc = proc.returncode
    status_msg = t(cfg, "log_finished_ok") if rc == 0 else t(cfg, "log_finished_err", v=rc)
    lines.append(status_msg)

    if cfg.get("notify_bell", True):
        sys.stdout.write("\a")
        sys.stdout.flush()

    add_history_entry(op_type, source, target, "Success" if rc == 0 else f"Failed ({rc})")

    h, w = stdscr.getmaxyx()
    stdscr.erase()
    draw_header(stdscr, t(cfg, "log_title"), w)
    log_h = max(0, h - 4)
    for idx, line in enumerate(lines[-log_h:]):
        safe_addstr(stdscr, 3 + idx, 2, line, w - 4)
    draw_footer(stdscr, t(cfg, "log_footer_done"), w, h)
    stdscr.refresh()
    while True:
        k = stdscr.getch()
        if k in (curses.KEY_ENTER, 10, 13, 27, ord('q'), 17):
            break

# ==========================================================================
# Curses Base Menus & Action Dialogs
# ==========================================================================

def run_menu(stdscr, title: str, items: list[str], footer: str, subtitle: str = "", start_index: int = 0, descriptions: list[str] = None) -> int | None:
    idx = min(start_index, len(items) - 1) if items else 0
    curses.curs_set(0)
    while True:
        stdscr.erase()
        h, w = stdscr.getmaxyx()
        draw_header(stdscr, title, w)
        row = 2
        if subtitle:
            for sline in subtitle.splitlines():
                safe_addstr(stdscr, row, 2, sline, w - 4, curses.A_DIM)
                row += 1
            row += 1

        for i, item in enumerate(items):
            y = row + i
            if y >= h - 4: break
            attr = HL_ATTR if i == idx else curses.A_NORMAL
            marker = "> " if i == idx else "  "
            safe_addstr(stdscr, y, 4, f"{marker}{item}", w - 8, attr)

        draw_footer(stdscr, footer, w, h)
        stdscr.refresh()

        key = stdscr.getch()
        if check_terminal_hotkey(stdscr, key):
            TERMINAL_OVERLAY.open(stdscr, load_config())
            continue
        if key in (curses.KEY_UP, ord('k')):
            idx = (idx - 1) % len(items) if items else 0
        elif key in (curses.KEY_DOWN, ord('j')):
            idx = (idx + 1) % len(items) if items else 0
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
        safe_addstr(stdscr, 5, 4, "> " + "".join(buf), w - 8)
        draw_footer(stdscr, footer, w, h)
        try:
            stdscr.move(5, min(w - 1, 6 + len(buf)))
        except curses.error: pass
        stdscr.refresh()

        key = stdscr.getch()
        if check_terminal_hotkey(stdscr, key):
            TERMINAL_OVERLAY.open(stdscr, load_config())
            continue
        if key in (27, 17):
            curses.curs_set(0); return None
        if key in (curses.KEY_ENTER, 10, 13):
            curses.curs_set(0); return "".join(buf)
        if key in (curses.KEY_BACKSPACE, 127, 8):
            if buf: buf.pop()
        elif 32 <= key <= 126:
            buf.append(chr(key))

def show_message(stdscr, title: str, lines: list[str], footer: str) -> None:
    curses.curs_set(0)
    while True:
        stdscr.erase()
        h, w = stdscr.getmaxyx()
        draw_header(stdscr, title, w)
        for i, line in enumerate(lines):
            safe_addstr(stdscr, 3 + i, 4, line, w - 8)
        draw_footer(stdscr, footer, w, h)
        stdscr.refresh()
        key = stdscr.getch()
        if check_terminal_hotkey(stdscr, key):
            TERMINAL_OVERLAY.open(stdscr, load_config())
            continue
        if key in (curses.KEY_ENTER, 10, 13, 27, ord('q'), 17):
            return

def screen_manual_preset_config(stdscr, cfg: dict, fields: dict) -> tuple[str, dict] | None:
    cat_idx = 0
    item_idx = 0
    in_items = False

    while True:
        stdscr.erase()
        h, w = stdscr.getmaxyx()
        draw_header(stdscr, t(cfg, "manual_title"), w)

        left_w = 26
        divider_x = left_w + 2

        # Draw left categories
        for i, cat in enumerate(DOWNLOAD_PRESET_CATEGORIES):
            y = 3 + i
            is_sel = (i == cat_idx)
            attr = HL_ATTR if (is_sel and not in_items) else (curses.A_BOLD if is_sel else curses.A_NORMAL)
            safe_addstr(stdscr, y, 2, f"{'► ' if is_sel else '  '}{t(cfg, cat['title_key'])}", left_w, attr)

        # Action category at bottom left
        is_act_sel = (cat_idx == len(DOWNLOAD_PRESET_CATEGORIES))
        safe_addstr(stdscr, 3 + len(DOWNLOAD_PRESET_CATEGORIES) + 1, 2, f"{'► ' if is_act_sel else '  '}[ Save / Start ]", left_w, HL_ATTR if is_act_sel else curses.A_BOLD)

        for y in range(2, h - 2):
            safe_addstr(stdscr, y, divider_x, "|", 1, curses.A_DIM)

        right_x = divider_x + 3
        right_w = w - right_x - 2

        # Render items
        if cat_idx < len(DOWNLOAD_PRESET_CATEGORIES):
            flds = DOWNLOAD_PRESET_CATEGORIES[cat_idx]["fields"]
            item_idx = min(item_idx, len(flds) - 1)
            for idx, fld in enumerate(flds):
                y = 3 + idx
                is_sel = (idx == item_idx and in_items)
                val = fields.get(fld["key"], "(default)")
                label = f"{fld['label']}: {val}"
                cli_hint = fld.get("cli", "")
                safe_addstr(stdscr, y, right_x, f"{'> ' if is_sel else '  '}{label}", right_w - len(cli_hint) - 3, HL_ATTR if is_sel else curses.A_NORMAL)
                safe_addstr(stdscr, y, w - len(cli_hint) - 4, f"[{cli_hint}]", len(cli_hint) + 2, curses.A_DIM)
        else:
            actions = ["Save as Default Preset", "Download Once (Run)", "Cancel"]
            item_idx = min(item_idx, len(actions) - 1)
            for idx, act in enumerate(actions):
                y = 3 + idx
                is_sel = (idx == item_idx and in_items)
                safe_addstr(stdscr, y, right_x, f"{'> ' if is_sel else '  '}{act}", right_w, HL_ATTR if is_sel else curses.A_NORMAL)

        draw_footer(stdscr, t(cfg, "footer_vertical_items" if in_items else "footer_vertical_tabs"), w, h)
        stdscr.refresh()

        key = stdscr.getch()
        if check_terminal_hotkey(stdscr, key):
            TERMINAL_OVERLAY.open(stdscr, cfg)
            continue

        if not in_items:
            max_cats = len(DOWNLOAD_PRESET_CATEGORIES) + 1
            if key in (curses.KEY_UP, ord('k')):
                cat_idx = (cat_idx - 1) % max_cats
                item_idx = 0
            elif key in (curses.KEY_DOWN, ord('j')):
                cat_idx = (cat_idx + 1) % max_cats
                item_idx = 0
            elif key in (curses.KEY_ENTER, 10, 13, curses.KEY_RIGHT, ord('l')):
                in_items = True
                item_idx = 0
            elif key in (27, ord('q')):
                return None
        else:
            max_len = len(actions) if cat_idx == len(DOWNLOAD_PRESET_CATEGORIES) else len(DOWNLOAD_PRESET_CATEGORIES[cat_idx]["fields"])
            if key in (curses.KEY_UP, ord('k')):
                item_idx = (item_idx - 1) % max_len
            elif key in (curses.KEY_DOWN, ord('j')):
                item_idx = (item_idx + 1) % max_len
            elif key in (27, curses.KEY_LEFT, ord('h')):
                in_items = False
            elif key in (curses.KEY_ENTER, 10, 13):
                if cat_idx == len(DOWNLOAD_PRESET_CATEGORIES):
                    return ("save" if item_idx == 0 else ("run" if item_idx == 1 else None), fields)
                fld = DOWNLOAD_PRESET_CATEGORIES[cat_idx]["fields"][item_idx]
                if fld["type"] == "bool":
                    fields[fld["key"]] = not fields.get(fld["key"], False)
                elif fld["type"] == "text":
                    val = text_input(stdscr, "Preset Config", fld["label"], t(cfg, "footer_input"), default=str(fields.get(fld["key"], "")))
                    if val is not None: fields[fld["key"]] = val.strip()
                elif fld["type"] == "choice":
                    c_idx = run_menu(stdscr, fld["label"], [c[1] for c in fld["choices"]], t(cfg, "footer_nav"))
                    if c_idx is not None: fields[fld["key"]] = fld["choices"][c_idx][0]

def screen_manage_download_presets(stdscr, cfg: dict) -> None:
    while True:
        presets = cfg.get("download_presets", [])
        items = [t(cfg, "preset_create_new")] + [f"{'[DEFAULT] ' if p['id']==cfg.get('default_download_preset') else ''}{p['name']}" for p in presets]
        sel = run_menu(stdscr, t(cfg, "presets_list_title"), items, t(cfg, "footer_nav"))
        if sel is None or sel == -1: return
        if sel == 0:
            fields = get_effective_default_fields(cfg)
            res = screen_manual_preset_config(stdscr, cfg, fields)
            if res and res[0] in ("save", "run"):
                name = text_input(stdscr, "Preset Name", "Enter name for new preset:", t(cfg, "footer_input"))
                if name:
                    p_obj = {"id": f"p{int(time.time()*1000)}", "name": name.strip(), "fields": res[1]}
                    cfg.setdefault("download_presets", []).append(p_obj)
                    save_config(cfg)
        else:
            p = presets[sel - 1]
            acts = ["Set as Default", "Delete Preset", "Back"]
            ai = run_menu(stdscr, p["name"], acts, t(cfg, "footer_nav"))
            if ai == 0:
                cfg["default_download_preset"] = p["id"]
                save_config(cfg)
            elif ai == 1:
                cfg["download_presets"] = [pr for pr in cfg["download_presets"] if pr["id"] != p["id"]]
                save_config(cfg)

def screen_video(stdscr, cfg: dict) -> None:
    url = text_input(stdscr, t(cfg, "video_title"), t(cfg, "video_prompt_url"), t(cfg, "footer_input"))
    if not url: return

    modes = [t(cfg, "download_mode_quick"), t(cfg, "download_mode_preset"), t(cfg, "download_mode_manual")]
    m = run_menu(stdscr, t(cfg, "video_title"), modes, t(cfg, "footer_nav"), subtitle=t(cfg, "download_mode_subtitle"))
    if m is None: return

    out_dir = parse_user_path(cfg.get("download_dir", DEFAULT_DOWNLOAD_DIR))
    out_dir.mkdir(parents=True, exist_ok=True)

    if m == 0:  # Quick Download
        def_id = cfg.get("default_download_preset")
        preset = next((p for p in cfg.get("download_presets", []) if p["id"] == def_id), None)
        if not preset:
            preset = {"id": "default", "name": "Default", "fields": get_effective_default_fields(cfg)}
        cmd = ["yt-dlp", *build_ytdlp_args_from_preset(preset, cfg, out_dir), url]
        run_with_log(stdscr, cfg, cmd, op_type="Download Video", source=url, target=str(out_dir))
    elif m == 1:  # Saved preset
        presets = cfg.get("download_presets", [])
        if not presets:
            show_message(stdscr, t(cfg, "video_title"), ["No presets saved yet."], t(cfg, "footer_message"))
            return
        pi = run_menu(stdscr, t(cfg, "video_title"), [p["name"] for p in presets], t(cfg, "footer_nav"))
        if pi is not None:
            cmd = ["yt-dlp", *build_ytdlp_args_from_preset(presets[pi], cfg, out_dir), url]
            run_with_log(stdscr, cfg, cmd, op_type="Download Video", source=url, target=str(out_dir))
    else:  # Manual
        fields = get_effective_default_fields(cfg)
        res = screen_manual_preset_config(stdscr, cfg, fields)
        if res:
            preset_obj = {"id": "manual", "name": "Manual", "fields": res[1]}
            cmd = ["yt-dlp", *build_ytdlp_args_from_preset(preset_obj, cfg, out_dir), url]
            run_with_log(stdscr, cfg, cmd, op_type="Download Video", source=url, target=str(out_dir))

def screen_convert(stdscr, cfg: dict) -> None:
    fpath = text_input(stdscr, t(cfg, "convert_title"), t(cfg, "convert_prompt_file"), t(cfg, "footer_input"))
    if not fpath: return
    p = parse_user_path(fpath)
    if not p.exists() or not p.is_file():
        show_message(stdscr, t(cfg, "convert_title"), [t(cfg, "convert_err_notfound", f=str(p))], t(cfg, "footer_message"))
        return

    presets = get_ordered_convert_presets(cfg)
    pi = run_menu(stdscr, t(cfg, "convert_title"), [pr["name_ru" if cfg.get("language")=="ru" else "name_en"] for pr in presets], t(cfg, "footer_nav"))
    if pi is None: return
    preset = presets[pi]
    out_file = p.parent / f"{p.stem}{preset['suffix']}.{preset['ext']}"
    cmd = ["ffmpeg", "-y", "-i", str(p), *preset["ffmpeg_flags"], str(out_file)]
    run_with_log(stdscr, cfg, cmd, op_type="Convert File", source=p.name, target=out_file.name)

def screen_batch_convert(stdscr, cfg: dict) -> None:
    folder = text_input(stdscr, t(cfg, "batch_title"), t(cfg, "batch_prompt_folder"), t(cfg, "footer_input"))
    if not folder: return
    p = parse_user_path(folder)
    if not p.exists() or not p.is_dir():
        show_message(stdscr, t(cfg, "batch_title"), [t(cfg, "convert_err_notfound", f=str(p))], t(cfg, "footer_message"))
        return

    files = [f for f in p.iterdir() if f.is_file() and f.suffix.lower() in (".mp4", ".mkv", ".mov", ".avi", ".webm", ".flv")]
    if not files:
        show_message(stdscr, t(cfg, "batch_title"), [t(cfg, "batch_no_files")], t(cfg, "footer_message"))
        return

    presets = get_ordered_convert_presets(cfg)
    pi = run_menu(stdscr, t(cfg, "batch_title"), [pr["name_ru" if cfg.get("language")=="ru" else "name_en"] for pr in presets], t(cfg, "footer_nav"))
    if pi is None: return
    preset = presets[pi]
    out_dir = p / f"converted_{preset['id']}"
    out_dir.mkdir(parents=True, exist_ok=True)
    for f in files:
        out_f = out_dir / f"{f.stem}{preset['suffix']}.{preset['ext']}"
        cmd = ["ffmpeg", "-y", "-i", str(f), *preset["ffmpeg_flags"], str(out_f)]
        run_with_log(stdscr, cfg, cmd, op_type=f"Batch ({f.name})", source=f.name, target=out_f.name)

def screen_trim(stdscr, cfg: dict) -> None:
    fpath = text_input(stdscr, t(cfg, "trim_title"), t(cfg, "trim_prompt_file"), t(cfg, "footer_input"))
    if not fpath: return
    p = parse_user_path(fpath)
    if not p.exists(): return
    start_t = text_input(stdscr, t(cfg, "trim_title"), t(cfg, "trim_prompt_start"), t(cfg, "footer_input"), default="00:00:00")
    if not start_t: return
    end_t = text_input(stdscr, t(cfg, "trim_title"), t(cfg, "trim_prompt_end"), t(cfg, "footer_input"))
    if end_t is None: return
    mode = run_menu(stdscr, t(cfg, "trim_title"), [t(cfg, "trim_mode_copy"), t(cfg, "trim_mode_reencode")], t(cfg, "footer_nav"))
    if mode is None: return
    out_f = p.parent / f"{p.stem}_trimmed{p.suffix}"
    cmd = ["ffmpeg", "-y", "-ss", start_t.strip()]
    if end_t.strip(): cmd += ["-to", end_t.strip()]
    cmd += ["-i", str(p)]
    if mode == 0: cmd += ["-c", "copy", str(out_f)]
    else: cmd += ["-c:v", "libx264", "-c:a", "aac", str(out_f)]
    run_with_log(stdscr, cfg, cmd, op_type="Trim Media", source=p.name, target=out_f.name)

def screen_probe(stdscr, cfg: dict) -> None:
    fpath = text_input(stdscr, t(cfg, "probe_title"), t(cfg, "probe_prompt_file"), t(cfg, "footer_input"))
    if not fpath: return
    p = parse_user_path(fpath)
    if not p.exists(): return
    cmd = ["ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", str(p)]
    try:
        res = subprocess.run(cmd, capture_output=True, text=True, check=False)
        if res.returncode == 0:
            data = json.loads(res.stdout)
            lines = [f"File: {p.name}", f"Duration: {float(data.get('format', {}).get('duration', 0)):.2f}s", "Streams:"]
            for s in data.get("streams", []):
                lines.append(f"  [{s.get('codec_type')}] {s.get('codec_name')} ({s.get('width', '')}x{s.get('height', '')})")
            show_message(stdscr, t(cfg, "probe_title"), lines, t(cfg, "footer_message"))
    except Exception as e:
        show_message(stdscr, t(cfg, "probe_title"), [f"Error: {e}"], t(cfg, "footer_message"))

def screen_audio(stdscr, cfg: dict) -> None:
    url = text_input(stdscr, t(cfg, "audio_title"), t(cfg, "video_prompt_url"), t(cfg, "footer_input"))
    if not url: return
    fi = run_menu(stdscr, t(cfg, "audio_title"), [a.upper() for a in AUDIO_FORMATS], t(cfg, "footer_nav"))
    if fi is None: return
    fmt = AUDIO_FORMATS[fi]
    out_dir = parse_user_path(cfg["download_dir"])
    cmd = ["yt-dlp", "-x", "--audio-format", fmt, "--audio-quality", "0", *get_proxy_args(cfg), "-o", str(out_dir / "%(title)s.%(ext)s"), *cookie_args(cfg), url]
    run_with_log(stdscr, cfg, cmd, op_type="Download Audio", source=url, target=str(out_dir))

def screen_playlist(stdscr, cfg: dict) -> None:
    url = text_input(stdscr, t(cfg, "playlist_title"), t(cfg, "playlist_prompt_url"), t(cfg, "footer_input"))
    if not url: return
    out_dir = parse_user_path(cfg["download_dir"])
    fields = get_effective_default_fields(cfg)
    preset_obj = {"id": "playlist", "name": "Playlist", "fields": fields}
    cmd = ["yt-dlp", "--yes-playlist", *build_ytdlp_args_from_preset(preset_obj, cfg, out_dir, for_playlist=True), url]
    run_with_log(stdscr, cfg, cmd, op_type="Download Playlist", source=url, target=str(out_dir))

def screen_subs(stdscr, cfg: dict) -> None:
    url = text_input(stdscr, t(cfg, "subs_title"), t(cfg, "video_prompt_url"), t(cfg, "footer_input"))
    if not url: return
    out_dir = parse_user_path(cfg["download_dir"])
    cmd = ["yt-dlp", "--skip-download", "--write-subs", "--write-auto-subs", "--sub-langs", cfg.get("sub_langs", "ru,en"), *get_proxy_args(cfg), "-o", str(out_dir / "%(title)s.%(ext)s"), *cookie_args(cfg), url]
    run_with_log(stdscr, cfg, cmd, op_type="Download Subtitles", source=url, target=str(out_dir))

def screen_info(stdscr, cfg: dict) -> None:
    url = text_input(stdscr, t(cfg, "info_title"), t(cfg, "video_prompt_url"), t(cfg, "footer_input"))
    if not url: return
    cmd = ["yt-dlp", "--no-playlist", "-j", *get_proxy_args(cfg), *cookie_args(cfg), url]
    try:
        res = subprocess.run(cmd, capture_output=True, text=True, check=False)
        if res.returncode == 0:
            d = json.loads(res.stdout)
            lines = [f"Title: {d.get('title')}", f"Uploader: {d.get('uploader')}", f"Duration: {d.get('duration_string')}", f"Views: {d.get('view_count')}"]
            show_message(stdscr, t(cfg, "info_title"), lines, t(cfg, "footer_message"))
    except Exception as e:
        show_message(stdscr, t(cfg, "info_title"), [f"Error: {e}"], t(cfg, "footer_message"))

def screen_cookies(stdscr, cfg: dict) -> None:
    cfile = cfg.get("cookies_file") or DEFAULT_COOKIES_FILE
    cookies = parse_cookies_file(cfile)
    lines = [f"Cookies file: {cfile}", f"Entries: {len(cookies)}"] + [f"  - {c['domain']}: {c['name']}" for c in cookies[:15]]
    show_message(stdscr, "Cookies", lines, t(cfg, "footer_message"))

def screen_history(stdscr, cfg: dict) -> None:
    if not HISTORY_PATH.exists():
        show_message(stdscr, t(cfg, "history_title"), [t(cfg, "history_empty")], t(cfg, "footer_message"))
        return
    entries = json.loads(HISTORY_PATH.read_text(encoding="utf-8"))[:20]
    lines = [f"[{e.get('time')}] {e.get('type')} ({e.get('status')})\n  {e.get('source')} -> {e.get('target')}" for e in entries]
    show_message(stdscr, t(cfg, "history_title"), lines, t(cfg, "footer_message"))

def screen_update(stdscr, cfg: dict) -> None:
    acts = [t(cfg, "update_via_pkg"), t(cfg, "update_via_pip"), t(cfg, "update_via_ytdlp"), t(cfg, "update_cancel")]
    c = run_menu(stdscr, t(cfg, "update_title"), acts, t(cfg, "footer_nav"))
    if c == 0:
        cmd = ["sudo", "pacman", "-Syu", "yt-dlp"] if shutil.which("pacman") else ["winget", "upgrade", "yt-dlp"]
        run_with_log(stdscr, cfg, cmd, op_type="Update yt-dlp")
    elif c == 1:
        run_with_log(stdscr, cfg, [sys.executable, "-m", "pip", "install", "--upgrade", "yt-dlp"], op_type="Update yt-dlp")
    elif c == 2:
        run_with_log(stdscr, cfg, ["yt-dlp", "-U"], op_type="Update yt-dlp")

# ==========================================================================
# Main TUI Loop with Tabbed Navigation
# ==========================================================================

MAIN_TABS = [
    {"id": "online", "title_key": "tab_online", "keys": [("menu_video", screen_video), ("menu_audio", screen_audio), ("menu_playlist", screen_playlist), ("menu_subs", screen_subs), ("menu_info", screen_info)]},
    {"id": "local", "title_key": "tab_local", "keys": [("menu_convert", screen_convert), ("menu_batch_convert", screen_batch_convert), ("menu_trim", screen_trim), ("menu_probe", screen_probe)]},
    {"id": "system", "title_key": "tab_system", "keys": [("menu_history", screen_history), ("menu_cookies", screen_cookies), ("menu_settings", screen_settings_vertical), ("menu_update", screen_update), ("menu_exit", None)]}
]

def main_tui(stdscr) -> None:
    cfg = load_config()
    apply_theme(cfg)
    cur_tab = 0
    cur_item = 0

    while True:
        stdscr.erase()
        h, w = stdscr.getmaxyx()
        draw_header(stdscr, t(cfg, "app_title"), w)

        # ASCII Art
        row = 2
        if h >= 24 and w >= 82:
            for line in ASCII_ART:
                safe_addstr(stdscr, row, max(2, (w - len(line)) // 2), line, w - 4, curses.A_BOLD)
                row += 1
            row += 1

        # Horizontal Tabs bar
        tab_x = 4
        for idx, tab in enumerate(MAIN_TABS):
            t_name = f" [ {t(cfg, tab['title_key'])} ] "
            safe_addstr(stdscr, row, tab_x, t_name, w - tab_x, HL_ATTR if idx == cur_tab else curses.A_DIM)
            tab_x += len(t_name) + 2
        row += 2

        # Menu items
        active_keys = MAIN_TABS[cur_tab]["keys"]
        cur_item = min(cur_item, len(active_keys) - 1)
        for idx, (label_key, _) in enumerate(active_keys):
            y = row + idx
            if y >= h - 3: break
            safe_addstr(stdscr, y, 6, f"{'> ' if idx == cur_item else '  '}{t(cfg, label_key)}", w - 12, HL_ATTR if idx == cur_item else curses.A_NORMAL)

        draw_footer(stdscr, t(cfg, "footer_nav"), w, h)
        stdscr.refresh()

        key = stdscr.getch()

        if check_terminal_hotkey(stdscr, key):
            TERMINAL_OVERLAY.open(stdscr, cfg)
            continue
        if key in (curses.KEY_LEFT, ord('h')):
            cur_tab = (cur_tab - 1) % len(MAIN_TABS)
            cur_item = 0
        elif key in (curses.KEY_RIGHT, ord('l')):
            cur_tab = (cur_tab + 1) % len(MAIN_TABS)
            cur_item = 0
        elif key in (curses.KEY_UP, ord('k')):
            cur_item = (cur_item - 1) % len(active_keys)
        elif key in (curses.KEY_DOWN, ord('j')):
            cur_item = (cur_item + 1) % len(active_keys)
        elif key in (curses.KEY_ENTER, 10, 13):
            k, func = active_keys[cur_item]
            if k == "menu_exit": return
            if func: func(stdscr, cfg)
        elif key in (27, ord('q')):
            return

def main() -> int:
    signal.signal(signal.SIGINT, signal.SIG_IGN)
    try: locale.setlocale(locale.LC_ALL, "")
    except Exception: pass

    if curses and sys.stdout.isatty() and sys.stdin.isatty():
        try:
            curses.wrapper(main_tui)
            return 0
        except curses.error:
            pass
    print("[!] Curses TUI unavailable in this terminal environment.")
    return 1

if __name__ == "__main__":
    sys.exit(main())