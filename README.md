# dstask-web (Go Web-UI für dstask)

Ein schlankes Web-UI zur Bedienung von `dstask` über den Browser. Implementiert in Go, mit Basic Auth, Multi-User via pro-User Repo-Zuordnung und ausführbar auf Windows.

## Features (MVP)
- Basic Auth (Benutzer/Passwort via BCrypt oder ENV-Fallback)
- Listen/Ansichten: `next`, `open`, `active`, `paused`, `resolved`
- Tags/Projekte: `show-tags`, `show-projects`
- Kontext anzeigen/setzen: `context`, `context none`
- Aufgaben anlegen: `dstask add <summary +tags project: due:>`
- Aktionen: `start`, `stop`, `done`, `remove`, `log`, `note`
- Sync: `dstask sync`
- HTML-Listenansicht über `?html=1` mit Inline-Aktionslinks

## Voraussetzungen
- Go >= 1.22
- `dstask` installiert (unter Windows z. B. `C:\tools\dstask.exe`)
- Git für das `.dstask`-Repo

## Build
```powershell
# Aus dem Projektwurzelverzeichnis
go mod tidy
mkdir bin 2>$null
go build -o bin/dstask-web.exe ./cmd/dstask-web
```

## Start (einfach, ENV-Fallback)
```powershell
$env:DSTWEB_USER='admin'
$env:DSTWEB_PASS='admin'
# optional: Pfad zu dstask.exe überschreiben
# $env:DSTWEB_DSTASK_BIN='C:\tools\dstask.exe'
./bin/dstask-web.exe
# Browser: http://localhost:8080/
```

## Konfiguration (`config.yaml`)
Siehe mitgelieferte `config.yaml` (Beispiel). Felder:
```yaml
dstaskBin: "C:\\tools\\dstask.exe"   # Pfad zu dstask.exe (Windows)
users:                                 # Optional; wenn leer, ENV-Fallback nutzen
  - username: "admin"
    passwordHash: "<bcrypt-hash>"      # BCrypt (z. B. Cost 10)
repos:                                 # username -> HOME oder direkt .dstask
  admin: "C:\\Users\\admin"         # oder: "C:\\Users\\admin\\.dstask"
```
- Wenn `users` fehlt/leer ist, werden `DSTWEB_USER`/`DSTWEB_PASS` genutzt.
- `repos` bestimmt je User den Arbeitsbereich:
  - Wenn Pfad ein HOME ist, verwenden wir `HOME/.dstask`.
  - Wenn Pfad bereits auf `.dstask` zeigt, wird dieses direkt genutzt.
- Zur Laufzeit kann `dstaskBin` via ENV überschrieben werden: `DSTWEB_DSTASK_BIN`.

### BCrypt-Hash erzeugen
- Empfohlen: kleines Go-Snippet (lokal, nicht Teil des Projekts):
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
Den generierten Hash in `config.yaml` als `passwordHash` eintragen.

## `.dstask`-Repo vorbereiten
Richte das Git-Repo im `.dstask`-Verzeichnis des jeweiligen Users ein. Entweder gemäß `repos.<user>` (falls dort `.dstask` steht) oder unter `<HOME>\.dstask`.

```powershell
# Beispiel: im .dstask-Verzeichnis
cd C:\Users\admin\.dstask
git init
git add .
git commit -m "init"
git remote add origin <REMOTE_URL>
git push -u origin master
```

Wenn `/sync` meldet „There is no tracking information for the current branch“, fehlt der Upstream:
```powershell
git branch --set-upstream-to=<remote>/<branch> master
# oder beim ersten Push:
# git push -u origin master
```

## Endpunkte
- `/` Startseite
- `/next`, `/open`, `/active`, `/paused`, `/resolved` (Plaintext)
  - HTML-Ansicht: `?html=1` (z. B. `/open?html=1`)
- `/tags`, `/projects`
- `/context` (GET zeigt, POST setzt oder löscht per `none`)
- `/tasks/new` (Form), `POST /tasks` (anlegen)
- `POST /tasks/{id}/{action}` mit `action` in `{start,stop,done,remove,log,note}`; für `note` Feld `note` im Body
- `/tasks/action` (Form-UI), `POST /tasks/submit`
- `/version`, `/sync` (GET Info, POST ausführen)

## Windows-spezifisches
- Der Server setzt `HOME` und `USERPROFILE` basierend auf `repos.<user>`, damit `dstask` sein Repo korrekt findet.
- Zeilenenden werden vereinheitlicht (CRLF→LF) bevor Ausgaben gerendert werden.

## Sicherheit
- Basic Auth mit BCrypt-Hashes oder ENV-Fallback
- Whitelist der `dstask`-Kommandos, keine freie CLI
- Zeitlimits: 5s für Listen, 10s mutierende Aktionen, 30s für Sync

## Roadmap/Nächste Schritte
- Optionale OIDC-Auth (z. B. Azure AD)
- Volle HTML-Tabellen mit Spalten (statt Textlisten)
- Bessere Fehlerdarstellung (Flash-Messages)
- Batch-Aktionen auf UI-Ebene

## Lizenz
Siehe ursprüngliche `dstask`-Lizenz für das CLI-Tool. Dieses Web-UI ist separat lizenziert (noch festzulegen).


