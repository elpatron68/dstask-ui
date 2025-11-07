### Release Notes – v0.1.5.2

- **Music Stream Player**
  - Per-task radio integration with persisted volume/mute state via `/music/tasks/{id}`.
  - `/music/proxy` buffers audio and strips ICY metadata for glitch-free playback.
  - New UI controls (search, name, URL) in `Tasks → New` and task edit forms.

- **Git Integration**
  - Optional `gitAutoSync` config/CLI behavior; auto-runs `dstask sync` after task changes.
  - `/sync` warns if the repository has uncommitted changes.
  - CLI flag `-listen` and tests for precedence: flag > env > config > default.

- **Configuration & Docs**
  - Config file stored under `~/.dstask-ui/config.yaml` (Windows: `%USERPROFILE%\.dstask-ui`).
  - README covers stream setup, proxy behavior, and CLI overrides.
  - Example `config.yaml` includes `gitAutoSync` toggle.

- **Testing**
  - Added coverage for config overrides, auto-sync, and listen address resolution.
  - Extended server endpoint tests to validate music inputs and auto sync behavior.

- **Miscellaneous**
  - Cleanup of unused helpers and staticcheck warnings.
  - Automatic sync logging (`Auto sync`) added to command log.
