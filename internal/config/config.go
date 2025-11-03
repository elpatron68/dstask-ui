package config

import (
    "errors"
    "os"
    "path/filepath"
    "strconv"

    "gopkg.in/yaml.v3"
)

type UserConfig struct {
    Username     string `yaml:"username"`
    PasswordHash string `yaml:"passwordHash"` // bcrypt hash
}

type LoggingConfig struct {
    Level string `yaml:"level"` // debug, info, warn, error
}

type UIConfig struct {
    ShowCommandLog bool `yaml:"showCommandLog"`
    CommandLogMax  int  `yaml:"commandLogMax"`
}

type Config struct {
    DstaskBin string                 `yaml:"dstaskBin"`
    Users     []UserConfig           `yaml:"users"`
    Repos     map[string]string      `yaml:"repos"` // username -> path to ~/.dstask or home dir
    Logging   LoggingConfig          `yaml:"logging"`
    UI        UIConfig               `yaml:"ui"`
}

func Default() *Config {
    return &Config{
        DstaskBin: defaultDstaskBin(),
        Users:     []UserConfig{},
        Repos:     map[string]string{},
        Logging:   LoggingConfig{Level: "info"},
        UI:        UIConfig{ShowCommandLog: true, CommandLogMax: 200},
    }
}

func defaultDstaskBin() string {
    // Windows Default laut Vorgabe
    return `C:\\tools\\dstask.exe`
}

// Load lädt eine optionale YAML-Datei. Wenn pfad leer oder Datei fehlt, werden Defaults geliefert.
func Load(path string) (*Config, error) {
    cfg := Default()
    if path == "" {
        path = "config.yaml"
    }
    data, err := os.ReadFile(path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            applyEnvOverrides(cfg)
            return cfg, nil
        }
        return nil, err
    }
    if err := yaml.Unmarshal(data, cfg); err != nil {
        return nil, err
    }
    applyEnvOverrides(cfg)
    return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
    if v := os.Getenv("DSTWEB_DSTASK_BIN"); v != "" {
        cfg.DstaskBin = v
    }
    if v := os.Getenv("DSTWEB_LOG_LEVEL"); v != "" {
        cfg.Logging.Level = v
    }
    if v := os.Getenv("DSTWEB_UI_SHOW_CMDLOG"); v != "" {
        if v == "0" || v == "false" || v == "False" { cfg.UI.ShowCommandLog = false } else { cfg.UI.ShowCommandLog = true }
    }
    if v := os.Getenv("DSTWEB_CMDLOG_MAX"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 { cfg.UI.CommandLogMax = n }
    }
}

// ResolveHomeForUsername bestimmt das HOME für dstask anhand der Repo-Konfiguration.
// Erwartet, dass Repos[username] entweder auf ~/.dstask oder auf das Home-Verzeichnis zeigt.
func ResolveHomeForUsername(cfg *Config, username string) (string, bool) {
    p, ok := cfg.Repos[username]
    if !ok || p == "" {
        return "", false
    }
    base := filepath.Base(p)
    if base == ".dstask" {
        return filepath.Dir(p), true
    }
    return p, true
}


