# CLI

`mediacli` без аргументов открывает TUI. Команды работают без TTY, подходят для
скриптов и используют то же ядро, конфигурацию, пресеты и историю. Версия — 1.0.0.
Electron для CLI не нужен. Ядро, TUI и HTTP-бэкенд написаны на Rust.

```sh
mediacli --help
mediacli trim --help
mediacli config options
```

Флаги можно ставить до и после позиционных аргументов. Пути с пробелами нужно
заключать в кавычки. Для имени, начинающегося с `-`, используйте `--`.
`--json` выдаёт JSON в stdout, журналы инструментов идут в stderr. При нескольких
последовательных загрузках/конвертациях каждая завершённая операция может выдавать
отдельный JSON-объект; с `--json` это JSON Lines.

По умолчанию медиаоперация ждёт завершения. Код `0` означает успех, `1` — ошибку
операции, `2` — неверные аргументы, `124` — тайм-аут, `130` — прерывание. Ctrl+C
отменяет локальные задачи; временные файлы безопасной замены исходника проходят
завершающую обработку перед выходом. `--timeout 10m` ограничивает выполнение или
ожидание очереди; проверка метаданных имеет собственные ограничения времени.

`--dry-run` выводит план без скачивания, кодирования или постановки задач в очередь.
Построение плана может читать метаданные через ffprobe, а склейка без перекодирования
сохраняет служебный список входов в каталоге конфигурации. Пароли прокси в командах
плана скрываются. Настройки и экспорт конфигурации содержат исходные значения.

## Мастер скачивания в TUI

Загрузка видео, плейлиста, аудио и обложки открывает пошаговый мастер:

1. Введите HTTP(S)-ссылку и нажмите Enter.
2. Выберите текущие настройки по умолчанию, сохранённый пресет или «Создать новый пресет».
   Новый пресет копирует текущие параметры скачивания, включая папку назначения,
   авторизацию, качество, форматы и субтитры. Настройки оформления и редактора
   в него не входят. В форме ↑/↓ выбирают поле, Enter открывает варианты,
   Ctrl+U очищает значение, F2 открывает выбор файла или папки, Ctrl+S продолжает.
3. Выберите «Скачать один раз» либо «Сохранить как новый пресет и скачать».
   Во втором случае мастер запросит название. Параметры также можно изменить
   перед запуском; исходный сохранённый пресет при этом остаётся прежним.

Esc возвращает на предыдущий шаг; на шаге ввода ссылки — отменяет мастер.
Разовое скачивание не меняет общие настройки и список пресетов. Сохранённый
пресет появляется в списке при следующем скачивании.

## Соответствие возможностям TUI

| TUI | CLI |
| --- | --- |
| Видео, плейлисты, ручные параметры | `download`, `video`, `--playlist`, `--fields`, `--set` |
| Аудиозагрузка | `download --audio-only --audio-format mp3` |
| Обложки | `thumbnail --format png` |
| FFprobe и инспектор | `probe`, `inspect` |
| Конвертация и встроенные профили | `convert`, `transcode`, `presets --kind convert` |
| Обрезка | `trim --start ... --end ... --mode copy/reencode` |
| Соотношение сторон | `aspect --ratio 9:16 --mode crop/letterbox/blur/stretch` |
| Добавить/извлечь дорожки | `mux`, `demux` |
| Склейка | `concat --mode filter/demuxer` |
| Звук, фильтры, субтитры, метаданные | `audio`, `filter`, `subtitle`, `metadata` |
| Размер кадра, FPS, стабилизация | `scale`, `fps`, `stabilize` |
| Alt+3 и Pipeline | `pipeline`, `editor`, `--project`, `--op`, `--save-project` |
| Предпросмотр результата | `pipeline --preview 5` |
| Размер результата | `size --size-mb 50 --two-pass` |
| Фрагменты и звуковая волна | `ranges --range ...`, `waveform` |
| Пакетная обработка | `batch` |
| Сохранённые наборы | `presets list/show/save/delete` |
| История | `history list/show/retry/delete/clear` |
| Очередь, отмена, журналы, повтор | `queue ... --server URL` |
| Настройки, темы, cookies, прокси | `config`, `config options`, `cookies` |
| Проверка зависимостей | `doctor` |
| Файлы в папке загрузок | `library` |

Навигация, отмена/повтор редактирования и палитра остаются интерактивными действиями
TUI. В CLI редактирование задаётся аргументами или JSON-проектом; повторяемость
обеспечивают сохранённые проекты и история команд.

## Загрузки

```sh
mediacli download 'https://example.com/video' --quality 1080 --preset standard_mp4
mediacli download 'https://example.com/playlist' --playlist --output-dir ./downloads
mediacli download 'https://example.com/video' --audio-only --audio-format flac
mediacli thumbnail 'https://example.com/video' --format png
mediacli download 'https://example.com/video' --cookies-mode browser --cookies-browser firefox
mediacli download 'https://example.com/video' --subs-enabled --sub-langs ru,en --embed-subs
mediacli download 'https://example.com/video' --transcode external --preset davinci_prores
```

`--preset` принимает ID сохранённого набора загрузки или встроенного видеопрофиля.
Все параметры ручной формы доступны через `--fields @download-fields.json` или
`--set KEY=VALUE`. Значения `--set` читаются как JSON, если это возможно:

```sh
mediacli download 'https://example.com/video' \
  --set embed_thumbnail=false --set retries=5 --set output_template='%(id)s.%(ext)s'
```

Частые поля имеют отдельные флаги: список в `download --help`. В режиме `external`
путь к каждому результату сообщает сам yt-dlp, после чего CLI запускает FFmpeg.
Для плейлистов обрабатывается каждый сообщённый файл. Нет выбора «самого свежего»
постороннего файла в папке. Внешняя конвертация выполняется локально; через демон
используйте `--transcode embedded`.

## Обработка файлов

```sh
mediacli probe "input video.mp4" --json
mediacli convert input.mp4 --preset standard_mp4 -o output.mp4
mediacli convert input.mp4 --vcodec libx265 --crf 23 --op-preset medium --container mkv
mediacli trim input.mp4 --start 00:00:10 --end 00:00:20 --mode reencode -o cut.mp4
mediacli aspect input.mp4 --ratio 9:16 --mode blur
mediacli audio input.mp4 --volume 0.5 --normalize
mediacli mux video.mp4 --extra-inputs audio.flac --shortest
mediacli demux video.mp4 --stream-type audio --stream-index 0 -o audio.m4a
mediacli subtitle input.mp4 --mode burn --subtitle-file captions.srt
mediacli subtitle input.mkv --mode extract --index 0 -o captions.srt
mediacli metadata input.mp4 --title 'Запись' --artist 'Автор'
mediacli scale input.mp4 --width 1280 --height 720
mediacli fps input.mp4 --fps 30
```

У каждой операции все поля модели операции доступны как флаги: подчёркивания заменены
на дефисы. Массивы можно передавать повторением флага или JSON-массивом, словари —
JSON-объектом. Строки передаются без JSON-кавычек. Параметр `input` субтитров называется
`--subtitle-file`; занятые общими опциями имена получают префикс `op-`, например
`--op-preset` задаёт скорость кодировщика, а `--preset` выбирает сохранённый профиль.
`--options JSON` или `--options @FILE` задаёт весь объект параметров; отдельные флаги
имеют приоритет. Неизвестные поля отклоняются.

Выходной файл по умолчанию получает новое имя. Существующий результат можно
заменить через `--overwrite`, но исходные и вспомогательные файлы защищены,
включая совпадение через жёсткие ссылки. После ошибки кодирования частичный
результат сохраняется; повтор из истории выбирает другое имя.

```sh
mediacli concat first.mp4 second.mp4 --mode demuxer -o joined.mp4
mediacli concat first.mp4 second.mp4 --mode filter -o joined.mp4
```

`demuxer` копирует потоки и требует совместимых входных форматов/кодеков.
`filter` перекодирует и требует согласованных размеров и состава потоков. Для
приведения разных файлов к общему формату сначала используйте `batch`.
Стабилизации требуется FFmpeg с фильтрами vidstab.

## Проекты и пакетная обработка

```sh
mediacli pipeline input.mp4 \
  --op 'trim={"start":"1","end":"5","mode":"reencode"}' \
  --op 'scale={"width":1280,"height":720}' \
  --op 'transcode={"container":"mp4","vcodec":"libx264","acodec":"aac","crf":"20"}' \
  --save-project project.json

mediacli pipeline --project project.json -o result.mp4
mediacli pipeline --project project.json --preview 5 -o preview.mp4
mediacli pipeline --project project.json --dry-run
mediacli editor --project editor --input another.mp4 --dry-run
mediacli pipeline --project pipeline --dry-run
```

Проекты имеют тот же JSON-формат, что черновики TUI: `inputs`, типизированные
`operations` и `output`. `--project editor` читает черновик Alt+3,
`--project pipeline` — черновик расширенного редактора. `--project -` читает stdin.
`--input` заменяет входы проекта; повторяемый `--op TYPE=JSON` добавляет операции.
`--save-project` сохраняет проект и завершает команду без экспорта. Если в проекте
сохранён размер результата, он применяется; `--size-mb` переопределяет его.

```sh
mediacli batch --directory ./input --preset my-media-preset --output-dir ./output --dry-run
mediacli batch --directory ./input --project project.json --output-dir ./output
mediacli batch a.mp4 b.mp4 --op 'transcode={"vcodec":"libx264","acodec":"aac","container":"mp4"}'
```

Проверяются все входы, совпадения выходных имён и существующие результаты. При
ошибках обычный запуск останавливается до постановки задач. `--skip-invalid`
запускает только прошедшие проверку файлы. Зависимости проходов сохраняются внутри
каждого файла, разные файлы используют общий лимит `bg_queue_max`.

## Размер и фрагменты

```sh
mediacli size input.mp4 --size-mb 50 --two-pass -o small.mp4
mediacli size input.mp4 --size-mb 50 --two-pass=false
mediacli ranges input.mp4 --range 00:00:10..00:00:20 --range 30..40
mediacli ranges input.mp4 --range 10..20 --range 30..40 --join -o joined.mp4
mediacli waveform input.mp4 --start 60 --columns 80 --json
```

Размер приблизительный: H.264/AAC, десятичные МБ, резерв 4% для контейнера.
Фрагменты экспортируются отдельно либо склеиваются в порядке перечисления;
используется точная обрезка с перекодированием. `waveform` выводит строку звуковой волны; с `--json` возвращает массив
амплитуд текущего окна длиной до 60 секунд.

## Настройки, наборы, история

```sh
mediacli config get
mediacli config options
mediacli config set language ru
mediacli config set theme nord
mediacli config set bg_queue_max 3
mediacli config export config-backup.json
mediacli config import config-backup.json
mediacli config reset

mediacli presets list --kind convert
mediacli presets list --kind video
mediacli presets list --kind media
mediacli presets save my-preset --kind media --file preset.json
mediacli presets show my-preset
mediacli presets delete my-preset

mediacli history list --search trim --json
mediacli history show 0
mediacli history retry 0
mediacli history delete 0
mediacli history clear
mediacli cookies list --file cookies.txt
mediacli cookies add .example.com session VALUE --file cookies.txt
mediacli doctor
mediacli library
```

Сохранение медиа- и загрузочных наборов принимает их JSON-формат из `presets show`;
формат проекта отличается наличием входов/выхода. Встроенные `convert` и `video`
профили доступны для чтения. Перед изменением через `config` сохраняется резервная
копия существующего файла. JSON-файлы с параметрами и заданиями имеют приватные права.
Индексы истории начинаются с нуля и выводятся в `history list`; при параллельном
завершении задач список может сдвинуться, поэтому перед удалением сверяйте запись.
`history delete --delete-file` удаляет медиа только внутри папки загрузок.

## Фоновая очередь

В отдельном терминале:

```sh
export MEDIACLI_TOKEN='your-local-token'
mediacli daemon --port 8181 --persist
```

Затем:

```sh
export MEDIACLI_SERVER=http://127.0.0.1:8181
export MEDIACLI_TOKEN='your-local-token'
mediacli convert input.mp4 --vcodec libx264 -o output.mp4 --background
mediacli queue list
mediacli queue logs 1
mediacli queue wait 1
mediacli queue cancel 1
mediacli queue retry 1 --background
mediacli queue cancel-group GROUP
```

Без `--background` команда ждёт результата задачи. В очередь можно отправлять
загрузки, пайплайны/операции и пакеты. `--server` также поддерживают probe, config,
медиа- и загрузочные пресеты, история и библиотека. Пути медиа должны быть доступны
демону. Thumbnail, size, ranges, waveform и редактирование cookie-файла выполняются
локально: для них не задавайте `--server`. Для отдельных локальных команд при
установленной переменной можно использовать `--server=`.

`--persist` сохраняет очередь демона в `daemon_queue.json`; `--queue-file FILE`
выбирает другой журнал и также включает сохранение. После аварийного перезапуска
незавершённые задачи отмечаются прерванными и запускаются только через `queue retry`.
При обычном завершении демон отменяет активные задачи и выполняет их завершающие
действия. Без `--persist` жизненный цикл прежний, что сохраняет поведение Electron.

## Исполняемые зависимости

В npm-пакетах и установщиках уже есть FFmpeg, ffprobe и Deno нужной архитектуры.
Ручной путь к FFmpeg в конфигурации имеет приоритет перед встроенным файлом.
При запуске из исходников без этих файлов используется системный PATH.

yt-dlp в сборки не входит. Перед первой загрузкой MediaCLI получает самостоятельный
исполняемый файл из стабильного официального релиза GitHub, проверяет SHA-256 и
версию. Python пользователю не нужен. Проверка обновлений выполняется при загрузке
не чаще раза в сутки; при недоступности GitHub используется сохранённая версия.

```sh
mediacli tools install
mediacli tools update
mediacli tools update --dry-run --json
```

Файлы хранятся в пользовательском каталоге данных: `%LOCALAPPDATA%/mediacli/tools/yt-dlp`
в Windows, `$XDG_DATA_HOME/mediacli/tools/yt-dlp` (обычно `~/.local/share/...`) в Linux
и `~/Library/Application Support/mediacli/tools/yt-dlp` в macOS. Обновления не меняют
PATH и не заменяют исполняемый файл уже работающей загрузки. При первой установке
нужен доступ к GitHub API и официальному CDN релизов. `tools` работает локально.
