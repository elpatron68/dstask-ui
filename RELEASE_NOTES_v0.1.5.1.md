### Release Notes – v0.1.5.1

- **Features**
  - dstask autodetection: `dstaskBin` is optional; binary resolved via PATH.
  - Config at `<HOME>/.dstask-ui/config.yaml`; auto-create with defaults on first start.
  - Home page shows local repo path when missing and provides clone form.
  - Startup logs the resolved repo directory per user.

- **UX/Internationalization**
  - All logs and error messages switched to English.

- **Path handling**
  - `~/...` is cross-platform: expands to `%USERPROFILE%\...` on Windows.
  - `repos` accepts HOME paths or direct `.dstask` directories; env vars expanded.

- **CI/CD**
  - Always upload coverage to Codecov (tokenless for public repos; token if provided).
  - Fixed Windows PowerShell interpolation for dstask download.
  - Fixed “secrets in if” workflow parser issue.
  - Staticcheck cleanups (ST1005, U1000, S1039).

- **Tests**
  - Updated to reflect new defaults; all tests passing.

- **Docs**
  - README updated (autodetection, path rules, first-start behavior).
  - `config.yaml` enriched with portable examples.

- **Breaking change**
  - `dstaskBin` default path removed; empty by default and uses PATH. Override via `DSTWEB_DSTASK_BIN` if needed.

- **Notes**
  - After first start, review `<HOME>/.dstask-ui/config.yaml` and adjust `repos`.
  - If `dstask` isn’t found, the releases page opens with OS-specific guidance.


