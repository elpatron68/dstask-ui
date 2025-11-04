# dstask-web (Go Web UI for dstask)

[![Test Coverage](https://img.shields.io/badge/coverage-33.2%25-0366d6.svg)](#)

A lightweight web UI to operate *[dstask](https://github.com/naggie/dstask)* from the browser. Implemented in Go, with Basic Auth, per-user repo mapping, and Windows support.
## Onboarding / Erste Schritte

Dieser Abschnitt führt dich durch Installation, Konfiguration und den Erst-Start inklusive Repository-Setup und Sync.

### Voraussetzungen
- `Go >= 1.22`
- `dstask` installiert (Linux/macOS über Paketmanager, Windows z. B. `C:\tools\dstask.exe`)
- `git` installiert (für das `.dstask` Git-Repository)

### Build
- Siehe Abschnitt „Build“ weiter unten. Kurzfassung (Linux/macOS):

```bash
# im Repository-Root
go mod tidy
mkdir -p bin
go build -o bin/dstask-web ./cmd/dstask-web
```

### Konfiguration (config.yaml)
- Beispiel siehe vorhandene `config.yaml`. Für Linux empfiehlt sich:

```yaml
dstaskBin: ""             # leer lassen, wenn dstask über PATH gefunden wird
listen: ":8080"
repos:
  admin: "~/.dstask"      # Nutzer -> HOME/.dstask
logging:
  level: "info"
```

- Alternativ kann `DSTWEB_DSTASK_BIN` zur Laufzeit gesetzt werden, um das `dstask`-Binary zu überschreiben.
- `repos.<user>` kann auf ein HOME zeigen (dann wird HOME/.dstask verwendet) oder direkt auf das `.dstask`-Verzeichnis.
- `~` und Umgebungsvariablen werden aufgelöst.

### Starten
```bash
export DSTWEB_USER=admin
export DSTWEB_PASS=admin
./bin/dstask-web
# Browser: http://localhost:8080/
```

### Verhalten beim ersten Start (Setup-Flow)
- **dstask-Binary-Prüfung**: Das Binary wird aus `config.yaml` oder aus dem `PATH` ermittelt. Ist es nicht auffindbar, startet der Server nicht und meldet den Fehler.
- **`.dstask`-Repo-Prüfung**: Pro Nutzer (gemäß `repos`) wird geprüft, ob das `.dstask`-Verzeichnis existiert.
  - Falls nicht vorhanden: Die App versucht, das `dstask`-Repository nicht-interaktiv zu initialisieren (Antwort „y“ auf die bekannte Rückfrage). Dies betrifft nur die lokale Struktur; Git-Remote wird dabei noch nicht gesetzt.
  - Auf der Startseite wird angezeigt, ob ein Git-Repository im `.dstask` existiert und ob ein Remote konfiguriert ist.
- **Kein Git-Repository vorhanden** (`~/.dstask/.git` fehlt): Die Startseite bietet ein Formular an, um eine Remote-URL zu klonen (`POST /sync/clone-remote`). Das Ziel ist `~/.dstask`.
- **Git-Repository vorhanden, aber kein Remote**: Die Startseite (und `POST /sync`) bieten ein Formular an, um `remote origin` zu setzen (`POST /sync/set-remote`).
- **Upstream/Tracking automatisch setzen**: Vor `sync` versucht die App, den Upstream zu setzen. Zuerst auf den aktuellen lokalen Branch (z. B. `master`), bei Bedarf wird der Remote-HEAD-Branch (z. B. `main`) ermittelt und verwendet. Falls das fehlschlägt, bleibt die Fehlermeldung sichtbar.

### Sync
- Button „Sync“ auf der Startseite oder Aufruf von `POST /sync` führt `dstask sync` aus.
- Häufige Git-Hinweise (z. B. fehlender Upstream) werden mit erklärender Meldung angezeigt.

### Endpunkte für Setup/Sync
- `POST /sync/clone-remote` – Klont ein Remote-Repository nach `~/.dstask` (leer oder neu).
- `POST /sync/set-remote` – Setzt `remote origin` für ein bestehendes Git-Repository in `~/.dstask`.
- `POST /sync` – Führt `dstask sync` aus (setzt best-effort Upstream, falls nötig).

### Troubleshooting
- „address already in use“ auf `:8080`: Anderen Prozess beenden oder `DSTWEB_LISTEN` setzen (z. B. `127.0.0.1:3000`).
- `dstask` nicht gefunden: `DSTWEB_DSTASK_BIN` setzen oder `dstask` installieren.
- Kein Upstream: Manuell setzen, z. B. (Branch ggf. anpassen):

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
dstaskBin: "C:\\tools\\dstask.exe"   # path to dstask.exe (Windows)
listen: ":8080"                        # listen address (e.g., ":8080" or "127.0.0.1:3000")
users:                                   # optional; if empty, env fallback is used
  - username: "admin"
    passwordHash: "<bcrypt-hash>"        # bcrypt (e.g., cost 10)
repos:                                   # username -> HOME or direct .dstask
  admin: "C:\\Users\\admin"           # or: "C:\\Users\\admin\\.dstask"
logging:
  level: "info"                          # debug | info | warn | error
ui:
  showCommandLog: true                   # show command footer by default
  commandLogMax: 200                     # ring buffer size per user
```
- Linux/macOS: set `dstaskBin` to your `dstask` path if it is not in PATH, e.g. `/usr/local/bin/dstask`.
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


