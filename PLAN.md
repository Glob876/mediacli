# MediaGUI — План превращения конвертера в полноценное рабочее место для медиа

> **Статус документа:** проектный план (draft v0.1) · Дата: 2026-09-17  
> **Цель:** из «простой конвертации» (текущие `ConvertPresets` + `ScreenAspectRatio`/`ScreenTrim`) сделать универсальную программу манипуляции медиа на базе `ffmpeg`/`ffprobe`/`yt-dlp`, где задачи автора (смена соотношения сторон, склейка аудио+видео) — лишь два из десятков сценариев, а не весь продукт.

---

## 1. Видение (Vision)

**MediaGUI = DaVinci Resolve Lite × HandBrake × LosslessCut × Audacity для ffmpeg, но без дублирования их ниш.**

- **Слоган:** «Всё, что умеет `ffmpeg`, но без запоминания флагов».
- **Пользователь открывает один файл или папку и может сделать с медиа *всё*, что позволяет `ffmpeg`:** перекодировать, перепаковать, обрезать без перекодирования, склеить, изменить соотношение сторон, заменить/миксовать аудио, наложить фильтры, вшить/выжечь субтитры, поправить метаданные, извлечь дорожки, нормализовать громкость, стабилизировать, деинтерлейсить — и сразу увидеть результат, поставить в очередь, повторить на пакете файлов.
- **Принцип:** Go-daemon — единственный источник правды (engine), Electron — тонкая оболочка. Любая фича доступна и из TUI, и из GUI, и по HTTP API (для скриптов).
- **Не-цель:** не повторять NLE-монтаж (таймлайн с ключ-кадрами как в Premiere) и не быть плеером-заменой VLC. Таймлайн — только для трима/конката/предпросмотра фрагментов.

---

## 2. Текущее состояние — аудит

### 2.1 Что уже есть (активы)

| Слой | Файлы | Что умеет |
|------|-------|-----------|
| **core** | `pkg/core/engine.go:92-135`, `pkg/core/engine.go:1731-1795` | `Config`, `PrepareFFmpegOutput`, `ProbeMedia` (ffprobe JSON), `FfmpegBin`/`ResolveFfmpeg`, `BuildYtDlpArgs`, `VideoPresets`/`ConvertPresets`, `BackgroundQueueManager` (workerLoop, AdoptLiveTask, SSE), `CheckDependencies` |
| **TUI** | `pkg/ui/screen_aspect.go:23-518`, `pkg/ui/screen_tools.go:13-114`, `pkg/ui/app.go:128-157` | Aspect (4 режима: crop/letterbox/blur/stretch, `calcTargetDimensions`, `buildAspectRatioFFmpegArgs`), Convert (10 пресетов), Trim (copy/re-encode), Probe, History/Settings |
| **daemon** | `pkg/daemon/daemon.go:48-70, 438-499, 514-644` | `GET /api/status|tasks|history|presets|convert/presets|library|browse|tools/ffmpeg|meta`, `POST /api/downloads|convert|tasks/:id/cancel`, SSE `tasks/:id/events`, библиотека с Range-стримом и кешем превью `library/thumb` |
| **Electron** | `electron/renderer/app.js:240-700`, `electron/renderer/index.html:47-310` | Главная (слоты загрузок, группировка), Library (сетка с превью), Convert (browse+форма), Settings (5 вкладок), History drawer, Lightbox, LogViewer, Palette/Console |

### 2.2 Узкие места (gap к «полноценному месту»)

1. **Конвертация = только фиксированные пресеты** (`pkg/core/engine.go:483-564`). Нет выбора кодека вручную, нет управления `CRF/preset/bitrate/resolution/fps/pix_fmt`.
2. **Aspect — единственный «творческий» инструмент.** Нет отдельной операции «склеить аудио+видео» без yt-dlp, нет трима без диалога, нет конката.
3. **Нет мульти-инпут операций.** Даемон `POST /api/convert` (`pkg/daemon/daemon.go:438-499`) принимает один `input`. Невозможно `ffmpeg -i video.mp4 -i audio.flac -c copy`.
4. **Нет filter-graph.** Каждый экран собирает `-vf` строкой. Нет чеков на совместимость фильтров, нет предпросмотра фильтра.
5. **Нет аудио-операций:** микширование, нормализация (loudnorm), смена частоты, фейды.
6. **Нет работы с субтитрами/метаданными/главами** вне yt-dlp `--embed-*`.
7. **Нет пакетной обработки:** нельзя выбрать 30 файлов и применить один пайплайн.
8. **Probe бедный:** `MediaProbeResult` (`pkg/core/engine.go:1720-1729`) не отдаёт `duration/bit_rate/tags/chapters`, фронт не показывает аудио-дорожки/субтитры.
9. **Предпросмотр статичен:** только `library/thumb` одним кадром; нет сравнения до/после, нет аудио-waveform.
10. **Очередь линейна:** нет зависимостей задач, нет dry-run/профиля, нет отмены части цепочки.

---

## 3. Принципы проектирования MediaGUI

1. **FFmpeg-first, UI-second.** Каждая операция — это *чистый* `ffmpeg`-конструкт (stream mapping, filter graph, codec opts). UI лишь генерирует `[]string` args. Сохраняем кнопку «Показать команду» везде.
2. **Поток «Выбери → Настрой → Предпросмотри → Поставь в очередь»** един для всех инструментов.
3. **Один движок — три лица:** TUI, Electron, HTTP API. Логика в `pkg/core`, никакого engine в `electron/`.
4. **Безопасные дефолты:** `-y` только после `PrepareFFmpegOutput`, защита от перезаписи исходника, dry-run по умолчанию для batch, все пути через `ParseUserPath` + `resolveLibraryPath`-guard.
5. **Прогресс честный:** парсить и `yt-dlp` %, и `ffmpeg` `frame/time/speed` (`pkg/core/engine.go:874-972`), отдавать через SSE.
6. **Расширяемость пресетами:** любой пайплайн сохраняется как пресет (JSON в `config.json:DownloadPresets`-аналоге).

---

## 4. Архитектура

```
┌─────────────────────────────────────────────────────────┐
│                    Electron Shell                        │
│  Workspace: Library · Inspector · Pipeline Builder ·     │
│  Preview Player · Queue · Presets · Console/Palette      │
└──────────────────────┬──────────────────────────────────┘
                       │ HTTP 127.0.0.1 (Bearer token)
┌──────────────────────▼──────────────────────────────────┐
│                  Go Daemon (pkg/daemon)                  │
│  /api/*  + SSE  + Range/Thumb  + Browse                 │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│                   pkg/core (engine)                      │
│  Config · QueueManager · Probe · MediaOps · Presets ·    │
│  FilterGraph · CommandBuilder · History                  │
└──────────────────────┬──────────────────────────────────┘
                       │ os/exec
          ┌────────────┼────────────┐
          ▼            ▼            ▼
       ffmpeg       ffprobe       yt-dlp
```

**Новые пакеты в `pkg/core`:**

- `media_probe.go` — расширенный `ProbeMedia` → `MediaInfo` (см. §5.1).
- `media_ops.go` — каждая операция как `Operation` interface: `Validate()`, `BuildArgs(info) []string`, `Describe()`.
- `filter_graph.go` — билдер `-vf`/`-af`/`-filter_complex` с проверкой совместимости.
- `pipeline.go` — цепочка операций (`Pipeline = []Operation`) → один `ffmpeg` вызов или N вызовов (когда нужен двухпроход).
- `presets_media.go` — `MediaPresets` (аналог `ConvertPresets`/`VideoPresets`).
- `batch.go` — пакетное применение пайплайна к списку файлов.

**Расширение `BackgroundQueueManager`:** поддержка цепочек (chain tasks), dry-run, оценка размера/времени, отмена группы.

---

## 5. Функциональные домены (что покрыть из ffmpeg)

Ниже — карта «хотелок ffmpeg → модуль MediaGUI». Покрытие стремимся к ~90% частых кейсов ffmpeg, 100% — невозможно (ffmpeg — это компилятор).

### 5.1 Инспекция и предпросмотр (фундамент для всего)

**Сейчас:** `ProbeMedia` только `streams[].codec_type/name,width,height` + `format.duration`.

**Стало:**

- Расширить `MediaProbeResult` (`pkg/core/engine.go:1720`) полями: `format.{bit_rate,size,tags}`, `streams[].{index,bit_rate,sample_rate,channels,language,disposition,tags,r_frame_rate,avg_frame_rate}`, `chapters[]`, `packets?`.
- Эндпоинт `GET /api/probe?path=` (рядом с `/api/browse`) → полный JSON для Inspector.
- **Inspector-панель** в Electron: вкладки Streams / Format / Chapters / Raw JSON. Возможность скопировать `ffprobe` команду.
- **Preview Player:** `<video>` уже есть (lightbox), добавить: выбор аудио-дорожки/субтитров, waveform для аудио (генерировать `ffmpeg -i input -filter_complex showwavespic`), покадровый степ, `ffplay`-подобный скруббер. Для фильтров — split-view до/после (генерировать 5с превью).

### 5.2 Транскодинг и ремукс (ядро конвертации)

**Сейчас:** 10 пресетов (`pkg/core/engine.go:483-564`), нет ручной настройки.

**Стало — модуль `Transcode`:**

- **Контейнер:** mp4/mkv/mov/webm/avi/m4v/mp3/flac/wav/m4a/opus/ogg (расширить `mediaExts` в `daemon.go:501`).
- **Видео-кодек:** `libx264/libx265/libsvtav1/libvpx-vp9/prores_ks/dnxhd/copy` + `crf/preset/profile/pix_fmt/b:v/maxrate/bufsize/g/keyint`.
- **Аудио-кодек:** `aac/opus/mp3/flac/pcm_s16le/copy` + `b:a/sample_rate/channels`.
- **Ремукс без перекода:** `-c copy` (быстрый ремукс, смена контейнера).
- **UI:** Pipeline Builder — блок «Encode» с live-подсказками (если `copy` + фильтр → ошибка). Пресеты остаются, но становятся *сохранёнными конфигурациями блока*.
- **Команда:** `ffmpeg -y -i in -c:v libx264 -crf 18 -preset medium -c:a aac -b:a 192k out`.

### 5.3 Соотношение сторон (твой кейс №1) — расширить

**Сейчас:** `ScreenAspectRatio` (`pkg/ui/screen_aspect.go:242-517`) с 4 режимами, `calcTargetDimensions`, `buildAspectRatioFFmpegArgs`.

**Стало — модуль `Aspect & Geometry`:**

- Сохранить 4 режима, добавить: **Rotate (90/180/270 + autorotate по `displaymatrix`)**, **Flip H/V**, **Crop с ручным `x:y:w:h`**, **Pad с выбором цвета padding**, **Scale с алгоритмом (`lanczos/bilinear`)**, **SAR/DAR fix**.
- Поддержка **анаморфотных** источников (показывать SAR/DAR).
- В даемоне: `POST /api/operations/aspect` с параметрами `ratio, mode, size, codec, rotate, flip`.
- Команда-пример (blur, уже есть): `-filter_complex "[0:v]scale=...:increase,crop=...,boxblur=25:5[bg];[0:v]scale=...:decrease[fg];[bg][fg]overlay=...`

### 5.4 Склейка аудио + видео (твой кейс №2) — отдельный первоклассный модуль

**Сейчас:** только внутри `yt-dlp --merge` или один `-i`. Нет UI для `ffmpeg -i v.mp4 -i a.m4a -c copy`.

**Стало — модуль `Mux / Demux`:**

- **Mux:** выбрать N видео + M аудио + K субтитров → один выход. Карта потоков `-map 0:v:0 -map 1:a:0 -map 2:s:0 -c copy -shortest`.
  - Под-кейсы: заменить аудиодорожку, добавить вторую аудиодорожку (языки), добавить субтитры, микшировать два аудио в один (`amix`).
- **Demux/Extract:** извлечь дорожки: `-map 0:a:0 -vn out.m4a`, `-map 0:s:0 out.srt`, аудио без видео.
- **UI:** Drag-&-Drop несколько файлов в Pipeline → блок Mux с таблицей `streams` (чекбоксы, язык, `disposition: default/forced`).
- **API:** `POST /api/operations/mux {inputs:[], maps:[], output}` + `POST /api/operations/demux`.
- **Безопасность:** проверка что все `inputs` внутри разрешённых директорий (расширить `resolveLibraryPath`).

### 5.5 Обрезка, склейка, конкатенация

**Сейчас:** `ScreenTrim` (`pkg/ui/screen_tools.go:47-87`) только один отрезок, `copy` vs `re-encode`.

**Стало — модуль `Cut & Concat`:**

- **Trim:** точный (`-ss` до `-i` для быстрого seek + `-ss` после для точности), по времени `HH:MM:SS.ms` или по кадрам, с/без перекода, с фейдами. Предпросмотр отрезка.
- **Split:** разрезать по главам/длите/таймкодам (`-segment`).
- **Concat:** два режима: `concat demuxer` (для однотипных файлов, `-f concat -safe 0 -i list.txt -c copy`) и `concat filter` (для разнотипных, `filter_complex concat=n=3:v=1:a=1` с перекодом). UI для сортировки списка.
- **Remove segment:** вырезать середину (инверсия trim через два отрезка + concat).

### 5.6 Масштаб, частота кадров, деинтерлейс, стабилизация

- **Scale/Pad** уже в Aspect, вынести как отдельный блок для произвольного `w:h` + `flags`.
- **FPS:** `-r`, `fps` filter, `minterpolate` для замедления/интерполяции.
- **Deinterlace:** `bwdif`, `yadif`.
- **Stabilize (vidstab):** двухпроход `vidstabdetect`+`vidstabtransform` — пример цепочки из 2 задач в очереди.

### 5.7 Аудио-манипуляции

- **Volume/loudnorm:** `-filter:a loudnorm`, `volume=0.5`, `acompressor`, `aecho`.
- **Tempo/pitch:** `atempo`, `rubberband` (если собран).
- **Resample/Channels:** `-ar 48000 -ac 2`.
- **Mix/Merge:** `amix`, `amerge`, `pan`.
- **Extract/Transcode audio-only** уже есть, добавить нормализацию и удаление тишины (`silenceremove`).
- **Визуализация:** показывать waveform/спектр до/после.

### 5.8 Видео-фильтры (creative)

Набор частых фильтров как чек-лист в Pipeline (каждый — блок с параметрами):

- `eq` (brightness/contrast/saturation), `hue`, `colorbalance`, `curves`, `vibrance`
- `unsharp`/`gblur`/`boxblur`, `denoise` (`hqdn3d`, `nlmeans`), `sharpen`
- `rotate`, `transpose`, `hflip/vflip`, `crop`, `scale`, `pad`
- `subtitles` (burn-in `subtitles=filename.srt:force_style=...`), `drawtext`, `overlay` (watermark)
- `fps`, `setpts` (speed up/slow down), `reverse`
- Комбинация → один `-vf` или `-filter_complex` (билдер следит за порядком).

### 5.9 Субтитры

- **Extract:** `-map 0:s:0 out.srt` + конвертация `srt↔ass↔vtt` (ffmpeg + `iconv`).
- **Embed (soft):** `-c:s mov_text` (mp4) / `srt` (mkv), язык/forced.
- **Burn-in (hard):** `-vf subtitles=...` (требует `libass`).
- **UI:** список дорожек из probe, предпросмотр субтитров, редактор стилей ASS.

### 5.10 Метаданные, обложки, главы

- **Metadata:** `-metadata title=... -metadata:s:v:0 language=eng` + удаление (`-map_metadata -1`).
- **Thumbnail/cover:** `-map 0:v -map 1:v -c:v:1 png -disposition:v:1 attached_pic` (аналог `--embed-thumbnail`, но локально).
- **Chapters:** импорт/экспорт `ffmetadata`, `split_chapters` уже есть в yt-dlp, добавить локальный `split by chapters`.

### 5.11 Пакетная обработка и пресеты

- Выбрать N файлов в Library/Browse → «Apply Pipeline to N files» → создаёт N задач в `GlobalQueue` (с `BGQueueMax` из `Config`).
- Шаблон имени выхода: `%(basename)s_converted.%(ext)s` + суффикс/префикс/папка.
- **MediaPresets** (`presets_media.go`): сохранять любой Pipeline как пресет (имя, описание, `operations[]`). Хранить в `config.json` рядом с `DownloadPresets` (`pkg/core/engine.go:56-58`).
- История (`HistoryEntry` в `engine.go:726-733`) расширяется полем `pipeline_id` + `command_preview`.

### 5.12 Очередь и наблюдаемость

- Группировка уже есть (slots/groups в `app.js`), добавить **Chain view**: задача-родитель → дочерние (например, vidstab 2 прохода).
- Dry-run: кнопка «Показать команду» без запуска.
- Оценка: `ffprobe` длительность → прогресс `time=` → ETA.
- Логи: уже SSE (`daemon.go:229-272`), добавить фильтр по уровню (`-v verbose`).

---

## 6. Концепт рабочего места (Workspace) в Electron

**Текущий layout:** Главная (пустая) + Library + Convert (browse+форма) + Settings + Drawer истории (`index.html:46-310`).

**Новый layout — 4 зоны, как в NLE, но упрощённо:**

```
┌──────────────┬─────────────────────────────┬──────────────────┐
│  Library /   │     Preview Player          │  Inspector       │
│  Browse      │  (video + waveform +        │  (Probe,        │
│  (дерево,    │   split before/after,       │   streams,      │
│   поиск,     │   выбор дорожек)            │   metadata)     │
│   drag-drop) │                             │                  │
├──────────────┴─────────────────────────────┴──────────────────┤
│  Pipeline Builder (цепочка блоков)                           │
│  [ Input ] → [ Trim 00:10-00:20 ] → [ Aspect 9:16 blur ] →  │
│  [ Mux + audio.flac ] → [ Encode H.264 CRF18 ] → [ Output ] │
│  + Add Operation ▾  ·  Save as Preset  ·  Show Command       │
├──────────────────────────────────────────────────────────────┤
│  Queue (слоты + live логи, как сейчас, но с chain)          │
└──────────────────────────────────────────────────────────────┘
```

- **Library/Browse** (`app.js: loadLibrary/loadBrowse`) остаётся, но добавляет чекбоксы для batch и показ `is_media` уже есть.
- **Inspector** — новая панель, питается от `GET /api/probe`.
- **Pipeline Builder** — центральная фича: список блоков, каждый — форма с параметрами, drag для порядка, валидация (например, `copy` несовместим с `scale`).
- **Preview** — генерировать 5-10с превью-пайплайна на лету (`ffmpeg -ss 5 -t 5 -i in -vf ... -f mp4 -movflags frag_keyframe+empty_moov pipe:1` → blob URL в `<video>`).
- **Queue** — существующие слоты (`index.html:118-129`, `app.js: renderSlots`) расширяются chain-группами.

**Навигация:** горячие клавиши остаются (`Shift+H` история, `Shift+I` настройки, `Shift+P` палитра, `Shift+T` консоль). Добавить `Alt+1..4` для фокуса зон.

---

## 7. API — расширения даемона

Сохранить существующие (`daemon.go:48-70`), добавить версионирование `/api/v1`:

- `GET /api/probe?path=` → `MediaInfo` (расширенный probe).
- `POST /api/pipeline/preview` → `{inputs, operations, t, duration}` → временный файл/pipe, возвращает `preview_url`.
- `POST /api/pipeline/run` → `{inputs, operations, output_template, options}` → `task_id` или `task_ids[]` (batch). Под капотом `pipeline.BuildArgs()`.
- `POST /api/operations/*` — тонкие обёртки для одного блока (удобно для TUI): `/aspect`, `/mux`, `/trim`, `/transcode`, `/filter`.
- `GET /api/pipeline/presets` / `POST /api/pipeline/presets` — CRUD для MediaPresets.
- `GET /api/presets` остаётся для yt-dlp пресетов; `GET /api/convert/presets` депрекейтнуть в пользу pipeline presets, но оставить для совместимости.

Все новые эндпоинты — loopback + Bearer, пути через `resolveLibraryPath`/`resolveConvertPath` (`daemon.go:424-433, 561-577`).

---

## 8. Модель данных (Go)

```go
// media_probe.go
type MediaInfo struct {
    Path     string            `json:"path"`
    Format   FormatInfo        `json:"format"` // duration, size, bit_rate, tags
    Streams  []StreamInfo      `json:"streams"`
    Chapters []ChapterInfo     `json:"chapters"`
}
type StreamInfo struct {
    Index      int               `json:"index"`
    CodecType  string            `json:"codec_type"` // video/audio/subtitle/attachment
    CodecName  string            `json:"codec_name"`
    Width      int               `json:"width,omitempty"`
    Height     int               `json:"height,omitempty"`
    BitRate    string            `json:"bit_rate,omitempty"`
    Duration   string            `json:"duration,omitempty"`
    Language   string            `json:"language,omitempty"`
    Disposition Disposition      `json:"disposition"`
    Tags       map[string]string `json:"tags,omitempty"`
    // video: r_frame_rate, avg_frame_rate, pix_fmt, profile
    // audio: sample_rate, channels, channel_layout
}

// pipeline.go
type Pipeline struct {
    ID         string      `json:"id"`
    Name       string      `json:"name"`
    Inputs     []string    `json:"inputs"`     // абсолютные пути
    Operations []Operation `json:"operations"` // полиморфный JSON (type field)
    Output     OutputSpec  `json:"output"`
}
type Operation interface {
    Type() string
    Validate(info *MediaInfo) error
    BuildArgs(ctx *BuildContext) ([]string, error) // добавляет к ffmpeg args
}
type OutputSpec struct {
    Path     string `json:"path"` // шаблон или явный путь
    Overwrite bool  `json:"overwrite"`
}

// Примеры операций:
type TranscodeOp struct { VCodec, ACodec, CRF, Preset, BitrateV, BitrateA, PixFmt string }
type AspectOp struct { RatioW, RatioH, Mode, SizePreset, Codec string; Rotate, FlipH, FlipV int }
type MuxOp struct { ExtraInputs []string; Maps []string; Shortest bool }
type TrimOp struct { Start, End string; Mode string } // copy|reencode
type ConcatOp struct { Inputs []string; Mode string } // demuxer|filter
type FilterOp struct { VF, AF string } // произвольный filter graph (advanced)
```

`CommandBuilder` собирает финальный `[]string{"ffmpeg","-y", "-i", ... , "-filter_complex", "...", ... , out}` и проверяет конфликты (`-c copy` + `-vf` → ошибка).

---

## 9. Дорожная карта (Roadmap) — итеративно, без big-bang

### Фаза 0 — Фундамент (1-2 недели)
- [ ] Расширить `ProbeMedia` → `MediaInfo`, эндпоинт `GET /api/probe`, Inspector read-only в TUI/Electron.
- [ ] Выделить `FfmpegBin`/`ResolveFfmpeg` уже есть; добавить `FfprobeBin` аналогично.
- [ ] Заложить `pkg/core/pipeline.go` + `Operation` интерфейс + `CommandBuilder` (пока 2 операции: Transcode, Trim).
- [ ] Добавить `GET /api/pipeline/presets` заглушку.

**Критерий готовности:** Inspector показывает все дорожки, `Show Command` работает для простого transcode.

### Фаза 1 — Твои кейсы как первоклассные фичи (2-3 недели) — **MVP MediaGUI**
- [ ] **Aspect** — портировать `buildAspectRatioFFmpegArgs` (`screen_aspect.go:473-518`) в `AspectOp` + API `POST /api/operations/aspect`, UI Pipeline блок.
- [ ] **Mux/Demux** — новый `MuxOp`/`DemuxOp`, UI для выбора 2+ файлов, команда с `-map`, тесты на `-shortest`.
- [ ] **Trim/Concat** — `TrimOp` (расширить `ScreenTrim`), `ConcatOp` (demuxer mode для MVP).
- [ ] Batch: выбрать N файлов → N задач в `GlobalQueue` (расширить `handleCreateConvert` логику).
- [ ] Electron: Pipeline Builder (вертикальный список блоков) + Preview 5с.

**Критерий:** пользователь может (а) поменять 16:9→9:16 любым из 4 режимов, (б) склеить `video.mp4 + audio.flac → out.mp4` без консоли, (в) обрезать 10 файлов одним пайплайном.

### Фаза 2 — Полноценный транскодинг и фильтры (3-4 недели)
- [ ] Ручной Transcode-блок (все кодеки/CRF/preset/bitrate/res/pix_fmt), remux `copy`.
- [ ] Filter-блок: 10 частых фильтров как чек-лист (eq, flip, rotate, deinterlace, denoise, blur, sharpen, fps, scale, pad).
- [ ] Субтитры: extract/embed/burn-in (проверка `libass` через `ffmpeg -filters`).
- [ ] Аудио: volume/loudnorm/atempo/resample/mix (amix).
- [ ] Пресеты: сохранение/загрузка Pipeline, импорт/экспорт JSON.

### Фаза 3 — Рабочее место (полировка, 2-3 недели)
- [ ] Preview split-view до/после, waveform, выбор дорожек в плеере.
- [ ] Queue chains (vidstab 2-pass), dry-run, ETA, отмена группы.
- [ ] Метаданные/обложки/главы — блок Metadata.
- [ ] TUI паритет: `ScreenPipeline` (аналог `ScreenAspectRatio` но универсальный).
- [ ] i18n для всех новых строк (ru/en/en-US, как в `pkg/ui/i18n.go` и `electron/renderer/locales/`).

### Фаза 4 — Продвинутое и нишевое (по мере спроса)
- [ ] Стабилизация vidstab, деинтерлейс bwdif, тонмаппинг HDR→SDR (`zscale`).
- [ ] Watermark/overlay/drawtext, удаление тишины, нормализация пакета.
- [ ] Плагины: пользовательские `filter_complex` пресеты, скрипты `deno` (уже в `CheckDependencies` optional).
- [ ] Экспорт queue как shell-скрипт (`ffmpeg` команды) для воспроизводимости.

---

## 10. UI/UX детали (Electron)

- **Дизайн-система** остаётся (`styles.css:1-14` — `--lime`, чёрный, острые углы). Новые блоки — те же `card`/`cselect`/`btn`.
- **Пустое состояние** (`index.html:111-115`) превращается в «Drop files to start Pipeline» + кнопка «New Pipeline».
- **Пресеты** (`view-presets` уже есть для загрузок) получают вторую вкладку «Media Pipelines».
- **Палитра команд** (`Shift+P`, `palette` в `index.html:436-448`) пополняется: `Aspect: 9:16 blur`, `Mux: add audio`, `Trim: 00:10-00:20`, `Transcode: H.264 CRF18`.
- **Консоль** (`Shift+T`) получает команды `probe <path>`, `pipeline run <preset> <files...>`, `queue clear`.
- **Горячие клавиши:** `Space` — play/pause preview, `I/O` — in/out для trim, `Ctrl+Enter` — запустить pipeline.

---

## 11. TUI паритет

- `tabs` в `pkg/ui/app.go:128-157` расширяются: вкладка `local` получает пункты `Pipeline`, `Mux`, `Batch` (рядом с `menu_aspect_ratio`, `menu_convert`, `menu_trim`).
- `ScreenPipeline` — универсальный экран, переиспользует `RunMenu`/`TextInput`/`RunWithLogHook` (`pkg/ui/runner.go`).
- Темы (`pkg/ui/theme.go`) и i18n (`pkg/ui/i18n.go`) — без изменений, новые ключи в `locales/`.

---

## 12. Тестирование и качество

- **Unit:** `CommandBuilder` — таблица «операция → ожидаемые args» (как `daemon_test.go` для `FfmpegBin`).
- **Probe:** golden JSON для 3 файлов (video+audio, audio-only, с субтитрами).
- **Интеграция:** `TestPipeline_E2E` на 2-секундном `testsrc` (генерит `ffmpeg -f lavfi -i testsrc=duration=2:size=128x128:rate=30`).
- **API:** `httptest` для новых `/api/probe|pipeline/*` (аналог `daemon_test.go`).
- **Fuzz:** `ParseUserPath`, `SplitShellFields` (`engine.go:177-231`) уже есть — добавить для `OutputSpec` template.
- **Ручное:** чек-лист из 15 сценариев (aspect каждый режим, mux 2 аудио, trim copy vs reencode, concat demuxer vs filter, burn-in srt, loudnorm, batch 10 файлов).

---

## 13. Риски и митигации

| Риск | Митигация |
|------|-----------|
| `ffmpeg` собран без `libass/vidstab` → burn-in/стабилизация падает | `CheckDependencies` + `ffmpeg -filters`/`-codecs` парсинг, серые чекбоксы с тултипом |
| `-c copy` + фильтр → неочевидная ошибка | `CommandBuilder.Validate` ловит до запуска, подсвечивает блок красным |
| Длинные `-filter_complex` → экранирование кавычек | Всегда `[]string` args, никогда shell-строка; `SplitShellFields` только для импорта |
| Batch перезапишет исходники | `PrepareFFmpegOutput` + `OverwriteOriginal` guard уже есть; для batch `overwrite=false` по умолчанию |
| SSE отваливается при 100 задачах | Throttle 250ms уже есть (`daemon.go:268`), добавить пагинацию `GET /api/tasks?status=running` |
| Разный `ffmpeg` в PATH у GUI vs терминала | `FfmpegBin`/`ResolveFfmpeg` уже решает; добавить `ffprobe` аналог |

---

## 14. Что делать дальше (следующий шаг после утверждения плана)

1. Утвердить приоритеты фаз (предлагается Фаза 0→1 как MVP).
2. Завести issues по каждому блоку Pipeline (по одному issue на операцию).
3. Начать с `media_probe.go` + `GET /api/probe` + Inspector — это разблокирует все остальные фичи (без probe нельзя валидировать операции).

---

## Приложение A. Карта ffmpeg → MediaGUI (выборочно)

| ffmpeg возможность | MediaGUI модуль | Статус сейчас |
|--------------------|-----------------|---------------|
| `-c:v libx264 -crf 18 -preset medium` | Transcode | частично (пресеты) |
| `-c copy` (ремукс) | Transcode (copy mode) | нет |
| `-ss/-to/-t` trim | Cut & Concat | частично (ScreenTrim) |
| `-f concat` demuxer/filter | Concat | нет |
| `scale/crop/pad/setsar` | Aspect & Geometry | частично (aspect) |
| `transpose/rotate/hflip/vflip` | Geometry | нет |
| `-map` (mux/demux) | Mux/Demux | нет (**твой кейс**) |
| `-filter_complex amix/amerge` | Audio Mix | нет |
| `loudnorm/volume/atempo` | Audio | нет |
| `subtitles` burn-in / `-c:s` soft | Subtitles | нет |
| `-metadata` / `attached_pic` / `chapters` | Metadata | нет |
| `bwdif/vidstab/zscale` | Advanced Filters | нет |
| `ffprobe -show_streams/chapters` | Inspector | частично |

*Итого сейчас покрыто ~15% частых сценариев ffmpeg; цель MediaGUI — 85-90% без ухода в монтажный таймлайн.*

---

## Приложение B. Связанные файлы для старта

- `pkg/core/engine.go:483-564` — ConvertPresets (образец для MediaPresets)
- `pkg/core/engine.go:1731-1743` — ProbeMedia (расширять)
- `pkg/core/engine.go:1752-1795` — FfmpegBin/ResolveFfmpeg (образец для FfprobeBin)
- `pkg/ui/screen_aspect.go:473-518` — buildAspectRatioFFmpegArgs (портировать в AspectOp)
- `pkg/ui/screen_tools.go:13-114` — ScreenConvert/ScreenTrim/ScreenProbe (референс UX)
- `pkg/daemon/daemon.go:438-499` — handleCreateConvert (образец для handlePipelineRun)
- `pkg/daemon/daemon.go:591-644` — handleLibraryThumb (образец для preview генерации)
- `electron/renderer/app.js:240-700` — i18n/backend/mock (куда втыкать новые API)
- `electron/renderer/index.html:142-177` — view-convert (прототип для Pipeline Builder)

