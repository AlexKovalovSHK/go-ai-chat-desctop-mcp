# 04_IMP — Build Guide
> Инструкция по сборке под все платформы
> Дата: 2025-03

---

## Локальная разработка (macOS → macOS)

### Требования

```bash
# 1. Go 1.21+
brew install go
go version  # go version go1.22.x darwin/arm64

# 2. XCode Command Line Tools (нужен Fyne)
xcode-select --install
xcode-select -p  # /Library/Developer/CommandLineTools

# 3. Fyne CLI (для упаковки в .app)
go install fyne.io/fyne/v2/cmd/fyne@latest
fyne version  # v2.5.x
```

### Запуск для разработки

```bash
cd gemini-chat

# Установить зависимости
go mod tidy

# Запуск (с API ключом из env для Phase 1)
GEMINI_API_KEY="AIza..." go run ./cmd/app/

# Запуск с детектором гонок (обязательно при работе с горутинами!)
GEMINI_API_KEY="AIza..." go run -race ./cmd/app/

# Проверка кода
go vet ./...
```

### Сборка .app для macOS

```bash
# Простая сборка бинарника
go build -ldflags="-s -w" -o gemini-chat ./cmd/app/

# Полноценный .app (с иконкой, Info.plist, подписью)
fyne package \
  -os darwin \
  -name "GeminiChat" \
  -appID "dev.geminichat.app" \
  -icon assets/icon.png    # PNG 256x256

# Результат: GeminiChat.app в текущей папке
# Установка: перетащить в /Applications
```

### Universal Binary для macOS (arm64 + amd64)

```bash
# Собрать под оба процессора
GOARCH=arm64 go build -ldflags="-s -w" -o gemini-chat-arm64 ./cmd/app/
GOARCH=amd64 go build -ldflags="-s -w" -o gemini-chat-amd64 ./cmd/app/

# Склеить в Universal Binary
lipo -create -output gemini-chat-universal gemini-chat-arm64 gemini-chat-amd64

# Или через fyne package — он делает universal автоматически на macOS
fyne package -os darwin -name GeminiChat -appID dev.geminichat.app
```

---

## Кросс-платформенная сборка

### Почему нельзя собрать всё с одного Mac

Fyne использует нативные GPU API:
- macOS → Metal (через CGo + obj-c)
- Windows → OpenGL/Direct3D (через CGo + windows headers)
- Linux → OpenGL (через CGo + X11 headers)

CGo требует компилятора и заголовков **целевой** платформы.
Поэтому кросс-компиляция Fyne-приложений возможна только через **Docker** или **CI**.

### Вариант A: fyne-cross (Docker на вашей машине)

`fyne-cross` — официальный инструмент от команды Fyne. Использует Docker-образы с нужными компиляторами.

```bash
# Установка
go install github.com/fyne-io/fyne-cross/cmd/fyne-cross@latest

# Требования: Docker Desktop запущен

# Сборка под Windows (с Mac)
fyne-cross windows \
  -arch amd64 \
  -name GeminiChat \
  -app-id dev.geminichat.app \
  ./cmd/app/
# → dist/windows-amd64/GeminiChat.exe

# Сборка под Linux (с Mac)
fyne-cross linux \
  -arch amd64 \
  -name GeminiChat \
  -app-id dev.geminichat.app \
  ./cmd/app/
# → dist/linux-amd64/GeminiChat

# macOS arm64 (на Intel Mac для Apple Silicon)
fyne-cross darwin \
  -arch arm64 \
  -name GeminiChat \
  -app-id dev.geminichat.app \
  ./cmd/app/
# → dist/darwin-arm64/GeminiChat.app
```

**Плюсы:** всё на одной машине, не нужен GitHub
**Минусы:** Docker должен быть запущен, большие образы (~2-4 GB)

### Вариант B: GitHub Actions (рекомендован для релизов)

Бесплатный план GitHub даёт 2000 минут CI в месяц — этого хватает.

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags: ['v*.*.*']   # Триггер: git tag v1.0.0 && git push --tags

permissions:
  contents: write      # Для создания GitHub Release

jobs:
  # ── macOS ──────────────────────────────────────────────────────
  build-macos:
    runs-on: macos-latest   # macOS 14, Apple Silicon
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Install Fyne CLI
        run: go install fyne.io/fyne/v2/cmd/fyne@latest

      - name: Build macOS Universal
        run: |
          fyne package \
            -os darwin \
            -name GeminiChat \
            -appID dev.geminichat.app \
            -icon assets/icon.png \
            ./cmd/app/
          zip -r GeminiChat-mac.zip GeminiChat.app

      - uses: actions/upload-artifact@v4
        with:
          name: mac
          path: GeminiChat-mac.zip

  # ── Windows ────────────────────────────────────────────────────
  build-windows:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Install Fyne CLI
        run: go install fyne.io/fyne/v2/cmd/fyne@latest

      - name: Build Windows exe
        run: |
          fyne package `
            -os windows `
            -name GeminiChat `
            -appID dev.geminichat.app `
            -icon assets/icon.png `
            ./cmd/app/
          Compress-Archive -Path GeminiChat.exe -DestinationPath GeminiChat-win.zip

      - uses: actions/upload-artifact@v4
        with:
          name: windows
          path: GeminiChat-win.zip

  # ── Linux ──────────────────────────────────────────────────────
  build-linux:
    runs-on: ubuntu-22.04
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: System dependencies (Fyne on Linux)
        run: |
          sudo apt-get update
          sudo apt-get install -y \
            libgl1-mesa-dev \
            xorg-dev \
            libx11-dev \
            libxrandr-dev \
            libxinerama-dev \
            libxcursor-dev \
            libxi-dev

      - name: Install Fyne CLI
        run: go install fyne.io/fyne/v2/cmd/fyne@latest

      - name: Build Linux binary
        run: |
          fyne package \
            -os linux \
            -name GeminiChat \
            -appID dev.geminichat.app \
            -icon assets/icon.png \
            ./cmd/app/
          tar -czf GeminiChat-linux.tar.gz GeminiChat

      - uses: actions/upload-artifact@v4
        with:
          name: linux
          path: GeminiChat-linux.tar.gz

  # ── GitHub Release ─────────────────────────────────────────────
  release:
    needs: [build-macos, build-windows, build-linux]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4

      - name: Create Release
        uses: softprops/action-gh-release@v2
        with:
          name: "GeminiChat ${{ github.ref_name }}"
          body: |
            ## Что нового в ${{ github.ref_name }}
            
            ### Установка
            - **macOS**: распакуйте zip, перетащите GeminiChat.app в /Applications
            - **Windows**: распакуйте zip, запустите GeminiChat.exe
            - **Linux**: распакуйте tar.gz, `chmod +x GeminiChat && ./GeminiChat`
            
            ### API Key
            Получите бесплатно: https://aistudio.google.com/app/apikey
          files: |
            mac/GeminiChat-mac.zip
            windows/GeminiChat-win.zip
            linux/GeminiChat-linux.tar.gz
          draft: false
          prerelease: false
```

### Как сделать релиз

```bash
# 1. Убедиться что всё работает
go test ./...
go vet ./...

# 2. Поставить тег
git add .
git commit -m "Release v1.0.0"
git tag v1.0.0
git push origin main --tags

# 3. GitHub Actions запустится автоматически
# 4. Через 10-15 минут: GitHub → Releases → v1.0.0
#    Там будут три файла для скачивания
```

---

## Размер бинарника

### Без оптимизации
```bash
go build -o gemini-chat ./cmd/app/
ls -lh gemini-chat  # ~25-35 МБ (включает debug symbols)
```

### С оптимизацией (для релиза)
```bash
go build -ldflags="-s -w" -o gemini-chat ./cmd/app/
ls -lh gemini-chat  # ~15-20 МБ

# Дополнительно: UPX-сжатие (опционально, замедляет старт на ~0.3 сек)
brew install upx
upx --best gemini-chat
ls -lh gemini-chat  # ~8-12 МБ
```

**Флаги `-ldflags="-s -w"`:**
- `-s` — убирает symbol table (отладочные символы)
- `-w` — убирает DWARF debug info
- Не влияет на производительность в рантайме

---

## Структура assets

```
assets/
├── icon.png          # 256×256 PNG — иконка приложения
├── icon@2x.png       # 512×512 PNG — для Retina (macOS)
└── icon.ico          # Windows ICO (fyne package генерирует сам из PNG)
```

### Минимальная иконка для разработки

```bash
# Создать placeholder иконку (серый квадрат) через ImageMagick
brew install imagemagick
convert -size 256x256 xc:"#4285F4" \
  -font Helvetica -pointsize 120 -fill white \
  -gravity center -annotate 0 "G" \
  assets/icon.png
```

---

## Полезные команды для отладки

```bash
# Запуск с детектором гонок (обязательно при горутинах)
go run -race ./cmd/app/

# Профилировщик памяти
go build -o gemini-chat ./cmd/app/
./gemini-chat &
go tool pprof http://localhost:6060/debug/pprof/heap

# Проверка что нет import fyne вне internal/ui/fyne/
grep -r "fyne.io" internal/ --include="*.go" | grep -v "internal/ui/fyne"
# Должно быть пусто

# Проверка линтером
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run ./...
```