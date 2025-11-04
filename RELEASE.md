# Release-Prozess

Dieser Leitfaden beschreibt den Release-Prozess und die Changelog-Konventionen für dstask-web.

## Versionierung
- SemVer (MAJOR.MINOR.PATCH)
- Beispiel: `v0.1.6`

## Changelog-Konvention
- Präfixe in Commit-Messages:
  - `feat:` neue Features
  - `fix:` Bugfixes
  - `docs:` Dokumentation
  - `test:` Tests
  - `ci:` CI/CD
  - `refactor:` interne Umstrukturierungen ohne Verhaltensänderung
  - `perf:`, `build:`, `chore:` optional
- Release Notes werden automatisch aus Commits generiert (GitHub Release Notes).

## Release erstellen
1. Stelle sicher, dass CI grün ist (Build + Tests + Linter/Staticcheck).
2. Erzeuge ein Tag und pushe es:
   ```bash
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```
3. Die CI (Workflow `CI`) erstellt beim Tag automatisch Artefakte und einen Release (mit Release Notes).
   - Falls das Tag vor dem Workflow-Setup erstellt wurde: Nutze den manuellen Workflow "Create Release (manual)" in GitHub Actions, um den Release nachträglich zu erzeugen.

## Coverage-Report (optional)
- Wenn `CODECOV_TOKEN` als Repository-Secret gesetzt ist, lädt der CI-Job die Coverage zu Codecov hoch.
- Badge in README aktualisiert sich je nach Codecov-Status (Badge-URL im README anpassen, wenn Codecov aktiv).

## Artefakte
- CI baut plattformübergreifende Builds (Linux, macOS, Windows; amd64/arm64) und hängt diese an Releases an.

## Hotfixes
- Hotfix-Branch von `master` abzweigen, patchen, CI durchlaufen lassen, neues Patch-Tag pushen.


