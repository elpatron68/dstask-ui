# dstask-web (Go Web UI for dstask)

[![Build](https://github.com/elpatron68/dstask-ui/actions/workflows/ci.yml/badge.svg)](https://github.com/elpatron68/dstask-ui/actions/workflows/ci.yml)
[![CodeQL](https://github.com/elpatron68/dstask-ui/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/elpatron68/dstask-ui/actions/workflows/codeql-analysis.yml)
[![Staticcheck](https://github.com/elpatron68/dstask-ui/actions/workflows/staticcheck.yml/badge.svg)](https://github.com/elpatron68/dstask-ui/actions/workflows/staticcheck.yml)
[![Codecov](https://codecov.io/gh/elpatron68/dstask-ui/branch/master/graph/badge.svg)](https://codecov.io/gh/elpatron68/dstask-ui)
![Go Version](https://img.shields.io/github/go-mod/go-version/elpatron68/dstask-ui)
![License](https://img.shields.io/github/license/elpatron68/dstask-ui)
![Release](https://img.shields.io/github/v/release/elpatron68/dstask-ui)
![Downloads](https://img.shields.io/github/downloads/elpatron68/dstask-ui/total)
![Last commit](https://img.shields.io/github/last-commit/elpatron68/dstask-ui)
![Issues](https://img.shields.io/github/issues/elpatron68/dstask-ui) ![PRs](https://img.shields.io/github/issues-pr/elpatron68/dstask-ui)
![Made with Go](https://img.shields.io/badge/Made%20with-Go-00ADD8?logo=go&logoColor=white)

A lightweight web UI to operate *[dstask](https://github.com/naggie/dstask)* from the browser. Implemented in Go, with Basic Auth, per-user repo mapping, and Windows support.
## Onboarding / Getting Started

This section walks you through installation, configuration, first launch, repo setup, and sync.

### Requirements
- Go >= 1.22
- `dstask` installed (Linux/macOS via package manager; Windows e.g. `C:\tools\dstask.exe`)
- `git` installed (for the `.dstask` Git repository)

### Build
- See the Build section below. Quick start (Linux/macOS):

```bash
# from repository root
go mod tidy
mkdir -p bin
go build -o bin/dstask-web ./cmd/dstask-web
```

### Configuration (config.yaml)
- See the provided `config.yaml` for an example. Minimal setup (PATH autodetection):

```yaml
listen: ":8080"
repos:
  admin: "~/.dstask"      # user -> HOME/.dstask
logging:
  level: "info"
# dstaskBin is optional; if omitted/empty, dstask is discovered via PATH
# dstaskBin: "/usr/local/bin/dstask"   # or on Windows: "C:\\tools\\dstask.exe"
```

#### Path specification and expansion rules

- **Home shortcut (`~/...`)**:
  - Linux/macOS: expanded to `$HOME/...`.
  - Windows: expanded to `%USERPROFILE%\\...` to keep configs portable across OSes.
- **Direct `.dstask` directory**:
  - If a repo path ends with `.dstask`, that directory is used directly (no extra appending).
- **Home directory path**:
  - If a repo path points to a user home (e.g., `~` or `C:\\Users\\alice`), the effective repo is `<HOME>/.dstask`.
- **Environment variables**:
  - All paths support env expansion (e.g., `$HOME`, `%USERPROFILE%`, `$APPDATA`).
- **Normalization**:
  - Paths are cleaned via platform-specific normalization.

Examples:

```yaml
repos:
  alice: "~/.dstask"              # portable; becomes %USERPROFILE%\\.dstask on Windows
  bob: "~"                        # resolves to <HOME> then <HOME>/.dstask is used
  carol: "C:\\Users\\carol\\.dstask"  # direct .dstask directory on Windows
  dave: "$HOME/.dstask"           # env var expansion on Linux/macOS
```

- Alternatively, set `DSTWEB_DSTASK_BIN` at runtime to override the `dstask` binary.
- `repos.<user>` may point to a HOME directory (then `HOME/.dstask` is used) or directly to the `.dstask` directory.
- `~` and environment variables are expanded.

### Start
```bash
export DSTWEB_USER=admin
export DSTWEB_PASS=admin
./bin/dstask-web
# Browser: http://localhost:8080/
```

### First-start behavior (setup flow)
- **dstask binary check**: The binary is taken from `config.yaml` or discovered via PATH. If not found, the server opens the releases page and returns an OS-specific instruction message to install/place `dstask` (see Troubleshooting).
- **`.dstask` repo check**: For each user (per `repos`) we check whether the `.dstask` directory exists.
  - If missing: the app tries to initialize the `dstask` repository non-interactively (auto-answer "y"). This creates the local structure only; no Git remote is configured.
  - The home page shows whether a `.git` repo is present and whether a remote is configured.
- **No Git repo present** (`~/.dstask/.git` missing): the home page shows a form to clone a remote (`POST /sync/clone-remote`) into `~/.dstask`.
- **Git repo present but no remote**: the home page and `POST /sync` show a form to set `remote origin` (`POST /sync/set-remote`).
- **Auto-set upstream (tracking)**: before `sync` the app attempts to set an upstream. It tries the current local branch first; if that fails, it detects the remote HEAD (e.g., `main`) and uses that. If it still fails, the error is displayed.

### Sync
- The "Sync" button on the home page (or `POST /sync`) runs `dstask sync`.
- Common Git hints (e.g., missing upstream) are displayed with guidance.

### Endpoints for setup/sync
- `POST /sync/clone-remote` – clone a remote repository into `~/.dstask` (new or empty directory).
- `POST /sync/set-remote` – set `remote origin` for an existing `.dstask` Git repository.
- `POST /sync` – run `dstask sync` (best-effort upstream auto-setup if needed).

### Troubleshooting
- "address already in use" on `:8080`: stop the other process or set `DSTWEB_LISTEN` (e.g., `127.0.0.1:3000`).
- `dstask` not found:
  - Install/Download `dstask` from `https://github.com/naggie/dstask/releases`, put it in your PATH (e.g., `/usr/local/bin/dstask` or `C:\\tools\\dstask.exe`) and make it executable.
  - Or set `DSTWEB_DSTASK_BIN` to the absolute path of the binary.
- No upstream: set it manually (adjust branch as needed):

```bash
git -C "$HOME/.dstask" branch --set-upstream-to=origin/main main
```


## Features (MVP)

- Basic Auth (bcrypt or env fallback)
- Views: `next`, `open`, `active`, `paused`, `resolved`
- Taxonomy: `show-tags`, `show-projects`
- Context: show/set via `context` / `context none`
- Add tasks: `dstask add <summary +tags project: due:>`
- Actions: `start`, `stop`, `done`, `remove`, `log`, `note`
- Sync: `dstask sync`
- HTML list views via `?html=1` with action buttons
- Rich HTML tables (`?html=1`) with sortable columns, status/priority badges, tag pills, and overdue highlighting
- Recent dstask commands footer with timestamps (configurable, per-user)
- Enhanced New Task form: select existing project or enter new, pick tags or add new, date picker for due
- Flash messages for success/error on actions
- Batch actions with multi-select (start/stop/done/remove/note)
- **Due filters**: Server-side filtering by due date (before/after/on/overdue) in HTML views
- **Templates**: List, create, edit, and delete task templates; create tasks from templates
- **Undo**: Roll back last action via `dstask undo` button in navbar
- **Open URLs**: Extract and display clickable URLs from task summaries and notes; automatic URL linkification in task lists

## Prerequisites

- Go >= 1.22
- `dstask` installed (on Windows e.g. `C:\tools\dstask.exe`)
- Git for the `.dstask` repo

## Build

### Linux/macOS

```bash
# from repository root
go mod tidy
mkdir -p bin
go build -o bin/dstask-web ./cmd/dstask-web
```

### Windows

```powershell
# from repository root
go mod tidy
mkdir bin 2>$null
go build -o bin/dstask-web.exe ./cmd/dstask-web
```

## Run (simple, env fallback)

### Linux/macOS

```bash
export DSTWEB_USER=admin
export DSTWEB_PASS=admin
# optional: override dstask binary path if not in PATH
# export DSTWEB_DSTASK_BIN=/usr/local/bin/dstask
./bin/dstask-web
# Browser: http://localhost:8080/
```

### Windows

```powershell
$env:DSTWEB_USER='admin'
$env:DSTWEB_PASS='admin'
# optional: override dstask.exe path
# $env:DSTWEB_DSTASK_BIN='C:\tools\dstask.exe'
./bin/dstask-web.exe
# Browser: http://localhost:8080/
```

## Configuration (`config.yaml`)

See the provided `config.yaml` for an example. Fields:

```yaml
# dstaskBin: "/usr/local/bin/dstask"      # optional; if omitted, PATH autodetection is used
listen: ":8080"                           # listen address (e.g., ":8080" or "127.0.0.1:3000")
users:                                      # optional; if empty, env fallback is used
  - username: "admin"
    passwordHash: "<bcrypt-hash>"           # bcrypt (e.g., cost 10)
repos:                                      # username -> HOME or direct .dstask
  admin: "~/.dstask"                       # or: "C:\\Users\\admin\\.dstask" on Windows
logging:
  level: "info"                             # debug | info | warn | error
ui:
  showCommandLog: true                      # show command footer by default
  commandLogMax: 200                        # ring buffer size per user
```
- Linux/macOS: if `dstask` is not in PATH, set `dstaskBin` (e.g. `/usr/local/bin/dstask`).
- You can override via env at runtime:
  - `DSTWEB_DSTASK_BIN` – absolute path to dstask
  - `DSTWEB_LISTEN` – listen address (e.g., `:8080` or `127.0.0.1:3000`)
  - `DSTWEB_LOG_LEVEL` – `debug|info|warn|error`
  - `DSTWEB_UI_SHOW_CMDLOG` – `true|false`
  - `DSTWEB_CMDLOG_MAX` – integer buffer size
- If `users` is missing/empty, `DSTWEB_USER`/`DSTWEB_PASS` are used.
- `repos` defines the workspace per user:
  - If the path is a HOME dir, `HOME/.dstask` is used.
  - If it points to `.dstask`, that directory is used directly.
- At runtime, `dstaskBin` can be overridden via `DSTWEB_DSTASK_BIN`.
- Listen address can be overridden via `DSTWEB_LISTEN` (e.g., `:8080` or `127.0.0.1:3000`).
- Logging level can be overridden via `DSTWEB_LOG_LEVEL`.
- Command log UI can be overridden via `DSTWEB_UI_SHOW_CMDLOG` (true/false) and `DSTWEB_CMDLOG_MAX` (int).

### Generate a bcrypt hash

Recommended: small Go snippet (local, not part of this project):
```go
package main
import (
    "fmt"
    "golang.org/x/crypto/bcrypt"
)
func main(){
    h,_ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
    fmt.Println(string(h))
}
```

```powershell
go run mkbcrypt.go
```

Place the generated hash into `config.yaml` under `passwordHash`.

## Prepare the `.dstask` repo
Initialize the Git repo in the user's `.dstask` directory. Either as configured via `repos.<user>` (if it points to `.dstask`) or under `<HOME>\.dstask`.

### Linux/macOS

```bash
# example: inside the .dstask directory
cd "$HOME/.dstask"
git init
git add .
git commit -m "init"
git remote add origin <REMOTE_URL>
git push -u origin master
```

### Windows

```powershell
# example: inside the .dstask directory
cd C:\Users\admin\.dstask
git init
git add .
git commit -m "init"
git remote add origin <REMOTE_URL>
git push -u origin master
```

If `/sync` shows “There is no tracking information for the current branch”, the upstream is missing:

```powershell
git branch --set-upstream-to=<remote>/<branch> master
# or on first push:
# git push -u origin master
```

## Endpoints

- `/` home
- `/next`, `/open`, `/active`, `/paused`, `/resolved` (plaintext)
  - HTML view: `?html=1` (e.g. `/open?html=1`)
  - Due filters: `?html=1&dueFilterType={before|after|on|overdue}&dueFilterDate=DATE`
- `/tags`, `/projects`
- `/context` (GET shows, POST sets or clears with `none`)
- `/tasks/new` (form), `POST /tasks` (create)
  - Template support: `?template={id}` to pre-select a template
- `POST /tasks/{id}/{action}` with action in `{start,stop,done,remove,log,note}`; for `note`, provide field `note`
- `GET /tasks/{id}/open` (display URLs extracted from task summary/notes)
- `/tasks/action` (form UI), `POST /tasks/submit`
- `POST /tasks/batch` (batch actions for selected IDs)
- `/templates` (GET list, POST create), `/templates/new` (form), `/templates/{id}/edit` (GET form, POST update), `POST /templates/{id}/delete`
- `POST /undo` (roll back last action)
- `/version`, `/sync` (GET info, POST run)

### Command log footer

- Visible on all HTML views by default; shows last 5 dstask commands (time, context, command).
- "Show more" expands to 20; add `all=1` query to show all.
- Toggle persists via cookie: links toggle between hide/show.

## Windows specifics

- The server sets `HOME` and `USERPROFILE` based on `repos.<user>` so `dstask` can find its repo.
- Line endings are normalized (CRLF→LF) before rendering output.

## Security

- Basic Auth via bcrypt hashes or env fallback
- Whitelist of allowed `dstask` commands, no arbitrary CLI
- Timeouts: 5s for lists, 10s for mutating actions, 30s for sync

## Roadmap / Next steps

- Optional OIDC auth (e.g., Azure AD)
- Extended batch actions (add/remove tags, set priority/project/due)
- Task edit form (web variant of `modify` command)
- Projects/Tags convenience links (click project/tag to filter)
- Help page (display `usage.md` in app)
- Security/UX polish (CSRF tokens, confirm dialogs, pagination)

## License

MIT License. See `LICENSE`.


