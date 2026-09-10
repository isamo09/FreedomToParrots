<div align="center">

<img src="docs/img/logo.png" width="140" height="140" alt="Freedom To Parrots">

# Freedom To Parrots

[![CI](https://github.com/isamo09/FreedomToParrots/actions/workflows/ci.yml/badge.svg)](https://github.com/isamo09/FreedomToParrots/actions/workflows/ci.yml)
[![Release](https://github.com/isamo09/FreedomToParrots/actions/workflows/release.yml/badge.svg)](https://github.com/isamo09/FreedomToParrots/actions/workflows/release.yml)
[![Latest release](https://img.shields.io/github/v/release/isamo09/FreedomToParrots)](https://github.com/isamo09/FreedomToParrots/releases/latest)
[![License](https://img.shields.io/badge/license-MIT-0D1117?style=flat-square)](LICENSE)

**Один исполняемый файл — панель управления и ядро зашифрованного
WebRTC-туннеля [olcrtc](https://github.com/openlibrecommunity/olcrtc)
вместе, под Windows, Linux, macOS, Termux/Android, FreeBSD и OpenBSD.**

</div>

Ничего отдельно устанавливать не нужно: ни Python, ни venv, ни рантайм —
только сам файл. Это переосмысление
[olcrtcPanel](https://github.com/isamo09/olcrtcPanel) (Python-панель +
отдельные бинарники ядра) в виде одного Go-бинарника на платформу, который
сам содержит нужное ядро и сам открывает свою консоль.

## Скачать

| Платформа | Файл |
|---|---|
| Windows | [FreedomToParrots-windows-amd64.exe](https://github.com/isamo09/FreedomToParrots/releases/latest/download/FreedomToParrots-windows-amd64.exe) |
| Linux (amd64) | [FreedomToParrots-linux-amd64](https://github.com/isamo09/FreedomToParrots/releases/latest/download/FreedomToParrots-linux-amd64) |
| Linux (arm64) / Termux | [FreedomToParrots-linux-arm64](https://github.com/isamo09/FreedomToParrots/releases/latest/download/FreedomToParrots-linux-arm64) |
| macOS (Apple Silicon) | [FreedomToParrots-darwin-arm64](https://github.com/isamo09/FreedomToParrots/releases/latest/download/FreedomToParrots-darwin-arm64) |
| macOS (Intel) | [FreedomToParrots-darwin-amd64](https://github.com/isamo09/FreedomToParrots/releases/latest/download/FreedomToParrots-darwin-amd64) |
| FreeBSD | [amd64](https://github.com/isamo09/FreedomToParrots/releases/latest/download/FreedomToParrots-freebsd-amd64) · [arm64](https://github.com/isamo09/FreedomToParrots/releases/latest/download/FreedomToParrots-freebsd-arm64) |
| OpenBSD | [amd64](https://github.com/isamo09/FreedomToParrots/releases/latest/download/FreedomToParrots-openbsd-amd64) · [arm64](https://github.com/isamo09/FreedomToParrots/releases/latest/download/FreedomToParrots-openbsd-arm64) |

Все ссылки — на **последний релиз** (`/releases/latest/download/…`), полный
список версий и контрольных сумм — во вкладке
[Releases](https://github.com/isamo09/FreedomToParrots/releases).

## Как это выглядит

Запускаете файл — из терминала или двойным кликом по `.exe` — и в том же
окне (или в новом окне консоли на Windows) сразу видите:

```
  Freedom To Parrots v0.1.0 — панель управления

  Панель:   http://192.168.1.77:8858/
  Панель:   http://127.0.0.1:8858/
  Пароль:   AFXi-K3CG-MJ3Z

  Клиенты (1 из 2 на связи):
  ────────────────────────────────────────────────────────────
  Имя                      Статус               Связь
  ────────────────────────────────────────────────────────────
  телефон                  на связи             только что
  старый роутер             переподключение…     никогда
  ────────────────────────────────────────────────────────────

  Проблемы:
  · «старый роутер»: не удаётся подключиться и связь ни разу не
    установилась — похоже, ссылка на звонок больше не действительна.

  Ctrl+C — остановить сервер и все туннели.
```

Экран перерисовывается на месте, а не листается в истории терминала.
Дальше вся работа — через веб-панель по адресу из консоли: добавление
устройств, QR-коды и строки подключения, лимиты трафика, логи. Внутри самой
панели есть страница `/download` с той же информацией, что ниже, плюс
кнопки скачивания.

## Запуск по платформам

**Windows** — скачайте `.exe`, запустите двойным кликом (SmartScreen на
неподписанный exe — «Подробнее → Выполнить в любом случае»). Закрыть окно
консоли = остановить сервер и все туннели.

**Linux / FreeBSD / OpenBSD** — статический бинарник без зависимостей:
```sh
chmod +x FreedomToParrots-linux-amd64
./FreedomToParrots-linux-amd64
```
Чтобы пережить закрытие терминала — `tmux`/`screen`, либо юнит `systemd`.

**macOS** — отдельные сборки под Intel (`amd64`) и Apple Silicon (`arm64`);
Gatekeeper спросит про неподписанный бинарник — «Системные настройки →
Конфиденциальность и безопасность → Всё равно открыть».

**Android / Termux** — `FreedomToParrots-linux-arm64` (Termux = обычный
aarch64 Linux userland). На Android 11+ без root SELinux может помешать
сбору ICE-кандидатов на самом устройстве — подробности и обходной путь на
странице `/download` панели.

Пароль от панели — случайный при первом запуске, всегда виден в консоли;
поменять можно в `data/settings.json` рядом с исполняемым файлом.

## Клиент — olcBOX

Каждое устройство в панели получает QR-код и строку подключения вида
`olcrtc://provider?transport@room#key$name`. Разобрать их умеет сторонний
GUI-клиент **olcBOX** ([alananisimov/olcbox](https://github.com/alananisimov/olcbox))
— его сборки лежат в том же релизе для удобства:

[Android (.apk)](https://github.com/isamo09/FreedomToParrots/releases/latest/download/Olcbox-1.0.127-android-universal-release.apk) ·
[Windows (portable .zip)](https://github.com/isamo09/FreedomToParrots/releases/latest/download/Olcbox-1.0.127-windows-amd64-portable.zip) ·
[macOS Apple Silicon (.dmg)](https://github.com/isamo09/FreedomToParrots/releases/latest/download/Olcbox-1.0.127-macos-arm64.dmg) ·
[macOS Intel (.dmg)](https://github.com/isamo09/FreedomToParrots/releases/latest/download/Olcbox-1.0.127-macos-amd64.dmg) ·
[Linux (.AppImage)](https://github.com/isamo09/FreedomToParrots/releases/latest/download/Olcbox-1.0.127-linux-amd64.AppImage) ·
[iOS (.ipa, неподписанный)](https://github.com/isamo09/FreedomToParrots/releases/latest/download/Olcbox-1.0.127-ios-unsigned.ipa) ·
[SHA256SUMS.txt](https://github.com/isamo09/FreedomToParrots/releases/latest/download/SHA256SUMS.txt)

Без стороннего приложения: тот же бинарник Freedom To Parrots содержит то
же ядро туннеля и умеет работать клиентом (режим `cnc`, поднимает локальный
SOCKS5-прокси) — формат конфигурации и разбор строки подключения описаны в
[документации ядра (uri.md)](https://github.com/openlibrecommunity/olcrtc/blob/master/docs/uri.md).

## Структура репозитория

```
cmd/fzp/                 точка входа: консоль + запуск HTTP-панели
internal/session/         жизненный цикл туннелей (Go-аналог panel.py)
internal/webui/            HTTP-обработчики и статика веб-панели
internal/webui/assets/     login.html, panel.html, download.html, style.css, panel.js
internal/console/          дашборд в терминале (без листания, ANSI+OSC8)
internal/corebin/          go:embed ядра olcrtc + распаковка при первом запуске
internal/store/            settings.json / sessions.json, определение папки данных
internal/procio/           учёт трафика по ОС (Linux и Windows — точно, остальные — n/a)
internal/security/         случайный пароль/токены, сравнение без тайминг-утечек
build/icon/                иконка (см. ниже) и её PNG-варианты по размерам
scripts/dev-build.sh(.ps1) локальная сборка с реальным ядром (для разработки)
.github/workflows/         ci.yml — компиляция на каждый push; release.yml — релизные сборки
```

## Сборка

Релизные сборки собирает GitHub Actions
(`.github/workflows/release.yml`): по тегу `vX.Y.Z` он берёт исходники
`cmd/olcrtc` из [openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)
(версия закреплена входом `core_ref`, по умолчанию `v0.0.1`), собирает
ядро под нужную платформу, встраивает через `go:embed` в
`internal/corebin` и собирает `cmd/fzp` — на выходе один файл на
платформу, без лишних шагов установки для пользователя. Иконка Windows-exe
встроена заранее через `go-winres` (см. `cmd/fzp/rsrc_windows_amd64.syso`,
пересобирается из `build/icon/icon-square.png` при смене иконки).

Локально (нужен только Go, без интернета для самой сборки):

```sh
# рядом должен быть checkout github.com/openlibrecommunity/olcrtc —
# либо в ../source (как в этом репозитории по соседству с olcrtcPanel),
# либо скрипт склонирует его сам во временную папку
./scripts/dev-build.sh          # Linux/macOS
scripts\dev-build.ps1           # Windows
```

`go build ./...` без этого шага тоже работает — но встраивает файл-заглушку
вместо настоящего ядра ровно для того, чтобы `go vet`/CI не требовали
интернета на каждый коммит; такой бинарник запустится, но покажет
предупреждение «ядро тоннеля недоступно» и не поднимет ни одной сессии.

## Отличия от olcrtcPanel (Python-версии)

- Один файл вместо интерпретатора + venv + отдельных бинарников ядра.
- Пароль от панели всегда генерируется случайно при первом запуске
  (в Python-версии по умолчанию можно было войти вообще без пароля).
- Убрана линуксовая фича привязки сессии к сетевому интерфейсу через
  собственный `uid` и policy routing (`ip rule`/`ip route`) — она требовала
  root и работала только на Linux, что плохо сочеталось с идеей «один
  бинарник — любая ОС». При необходимости вернуть — это был бы отдельный,
  явно Linux-only режим.
- Учёт трафика/CPU по процессу сейчас точный на Linux и Windows; на
  macOS/BSD сессии работают полностью, просто колонка трафика показывает
  «недоступно» вместо числа.

## Авторство

Ядро туннеля (`cmd/olcrtc`) разрабатывает **zarazaex**,
[openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc).
Клиент **olcBOX** — сторонний проект,
[alananisimov/olcbox](https://github.com/alananisimov/olcbox). Freedom To
Parrots — обвязка вокруг ядра olcrtc: консоль, панель и сборка в один файл.
