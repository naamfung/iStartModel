# iStartModel Agent Guide

## Overview

iStartModel is a GUI launcher for large language model engines like LlamaServer, with centralized configuration management. It is built with Go and the Fyne GUI framework, targeting Windows with a hidden console window (`windowsgui`).

## Project Structure

```
C:/iStartModel/
├── app.go              # Main application logic and GUI implementation
├── build.sh            # Build script for Windows GUI executable
├── config.yaml         # Configuration file with model schemes and settings
├── go.mod              # Go module definition (Go 1.26.4)
├── go.sum              # Go dependencies checksum
├── LICENSE             # License file
├── README.md           # Project documentation
├── .gitignore          # Git ignore rules
└── tmpl/               # Jinja chat templates for model agents
    ├── Qwen-Agentic-EN.jinja
    ├── Qwen-Agentic-HONS.jinja
    ├── Qwen-Agentic-HONT.jinja
    ├── fixed_for_froggeric_v19_chat_template.jinja
    └── fixed_for_froggeric_v20_chat_template.jinja
```

## Essential Commands

### Build

Build the Windows GUI executable (no console window):

```bash
./build.sh
# Or manually:
go build -ldflags '-H windowsgui -extldflags "-Wl,--subsystem,windows"'
```

### Run

After building, run the `.exe` file directly. The application is a GUI tray app that manages LlamaServer instances.

## Configuration Structure

The `config.yaml` file defines the application configuration:

- `llamaServer`: Path to the llama-server executable
- `model`: Default model path
- `template`: Default chat template path
- `alias`: Default alias
- `current`: Current active scheme name
- `schemes`: Map of scheme configurations, each with:
  - `alias`: Scheme alias name
  - `note`: Performance/configuration notes
  - `command`: Command arguments for the server
  - `useCustomEngine`: Use custom engine executable (from command)
  - `useCustomTemplate`: Use custom template
  - `useCustomCommand`: Use command exactly as specified, no拼接 (concatenation)

### Command Building Modes

The `buildCommand` function in `app.go` supports three modes:

1. **Custom Command Mode** (`useCustomCommand: true`): Returns the command exactly as specified after normalizing spaces.
2. **Custom Engine Mode** (`useCustomEngine: true`): Parses the first token as the engine executable, then appends model, template, and alias arguments as needed.
3. **Normal Mode** (default): Uses the global `llamaServer` executable and prepends it to the command, then appends model, template, and alias arguments.

## Code Patterns and Conventions

### Configuration Loading

- Config is loaded from `config.yaml` using `gopkg.in/yaml.v3`
- If `config.yaml` doesn't exist, a default config is created with an `example` scheme
- Command strings are normalized using `normalizeSpaces()` which replaces full-width spaces and collapses multiple spaces

### Command Parsing

- `buildCommand()` constructs the final server command based on scheme settings
- `splitCommand()` parses command strings into argument arrays, respecting single and double quotes
- Quote handling: state machine tracks `stateNone`, `stateDoubleQuote`, `stateSingleQuote`

### GUI Implementation

- Uses Fyne v2 for cross-platform GUI
- Implements system tray functionality using `fyne.io/systray`
- Configuration changes are saved back to `config.yaml` via `saveConfig()`

## Jinja Templates

The `tmpl/` directory contains tested Jinja chat templates for agent workloads:

- `Qwen-Agentic-EN.jinja` - English agent template
- `Qwen-Agentic-HONS.jinja` - Chinese agent template (HONS)
- `Qwen-Agentic-HONT.jinja` - Chinese agent template (HONT)

These templates are recommended for Qwen 3.5/3.6 models or derived models, as official templates may have issues in diverse real-world agent environments.

## Gotchas

1. **Build target is Windows-only**: The build script uses `-H windowsgui -extldflags "-Wl,--subsystem,windows"` which creates a Windows GUI executable without a console window.

2. **Command string normalization**: All command strings go through `normalizeSpaces()` which converts full-width spaces (`\u3000`) to regular spaces and collapses multiple spaces.

3. **Quote-aware command splitting**: The `splitCommand()` function uses a state machine to properly handle arguments with embedded spaces inside quotes. This is critical for paths with spaces or complex command arguments.

4. **Custom command mode bypasses all拼接**: When `useCustomCommand: true`, the command is returned exactly as specified (after space normalization), without adding model, template, or alias arguments.

5. **Engine mode vs Normal mode**: In `useCustomEngine: true` mode, the first token is treated as the engine executable, and the rest are treated as extra arguments. The function checks for existing `--model`, `--chat-template-file`, and `--alias` flags to avoid duplication.

## Dependencies

- `fyne.io/fyne/v2 v2.7.4` - GUI framework
- `gopkg.in/yaml.v3 v3.0.1` - YAML parsing
- `fyne.io/systray v1.12.1` - System tray integration (indirect)
