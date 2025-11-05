package config

import (
    "errors"
    "os"
    "path/filepath"
    "runtime"
    "strconv"
    "strings"

    applog "github.com/elpatron68/dstask-ui/internal/log"
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
	DstaskBin string            `yaml:"dstaskBin"`
	Listen    string            `yaml:"listen"` // listen address (e.g., ":8080")
	Users     []UserConfig      `yaml:"users"`
	Repos     map[string]string `yaml:"repos"` // username -> path to ~/.dstask or home dir
	Logging   LoggingConfig     `yaml:"logging"`
	UI        UIConfig          `yaml:"ui"`
}

func Default() *Config {
	return &Config{
        DstaskBin: defaultDstaskBin(),
		Listen:    ":8080",
		Users:     []UserConfig{},
		Repos:     map[string]string{},
		Logging:   LoggingConfig{Level: "info"},
		UI:        UIConfig{ShowCommandLog: true, CommandLogMax: 200},
	}
}

func defaultDstaskBin() string {
    // Kein Default-Pfad mehr: wir versuchen PATH-Autodetektion beim Start
    return ""
}

// Load lädt eine optionale YAML-Datei. Wenn pfad leer oder Datei fehlt, werden Defaults geliefert.
func Load(path string) (*Config, error) {
    cfg := Default()

    // Standardpfad: <HOME>/.dstask-ui/config.yaml (Windows: %USERPROFILE%\.dstask-ui)
    if path == "" {
        home, err := os.UserHomeDir()
        if err == nil && home != "" {
            baseDir := filepath.Join(home, ".dstask-ui")
            if err := os.MkdirAll(baseDir, 0755); err == nil {
                path = filepath.Join(baseDir, "config.yaml")
            } else {
                // Fallback: Arbeitsverzeichnis
                path = "config.yaml"
            }
        } else {
            // Fallback: Arbeitsverzeichnis
            path = "config.yaml"
        }
    }

    data, err := os.ReadFile(path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            // Datei existiert nicht: Defaults anwenden und Datei anlegen
            applyEnvOverrides(cfg)
            if err := writeDefaultConfigFile(path, cfg); err != nil {
                return nil, err
            }
            applog.Infof("Konfigurationsdatei wurde mit Default-Werten angelegt: %s", path)
            applog.Infof("Hinweis: Passen Sie bei Bedarf `repos` und weitere Felder in %s an.", path)
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

func writeDefaultConfigFile(path string, cfg *Config) error {
    // Elternverzeichnis sicherstellen
    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        return err
    }
    // YAML schreiben
    out, err := yaml.Marshal(cfg)
    if err != nil {
        return err
    }
    return os.WriteFile(path, out, 0644)
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("DSTWEB_DSTASK_BIN"); v != "" {
		cfg.DstaskBin = v
	}
	if v := os.Getenv("DSTWEB_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("DSTWEB_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("DSTWEB_UI_SHOW_CMDLOG"); v != "" {
		if v == "0" || v == "false" || v == "False" {
			cfg.UI.ShowCommandLog = false
		} else {
			cfg.UI.ShowCommandLog = true
		}
	}
	if v := os.Getenv("DSTWEB_CMDLOG_MAX"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.UI.CommandLogMax = n
		}
	}
}

// ResolveHomeForUsername bestimmt das HOME für dstask anhand der Repo-Konfiguration.
// Erwartet, dass Repos[username] entweder auf ~/.dstask oder auf das Home-Verzeichnis zeigt.
func ResolveHomeForUsername(cfg *Config, username string) (string, bool) {
    p, ok := cfg.Repos[username]
    if !ok || p == "" {
        return "", false
    }

    // Expand ~ und Umgebungsvariablen
    p = expandUserPath(p)
    p = os.ExpandEnv(p)
    p = filepath.Clean(p)

    base := filepath.Base(p)
    if base == ".dstask" {
        return filepath.Dir(p), true
    }
    return p, true
}

// expandUserPath ersetzt führendes "~" durch das Home-Verzeichnis des aktuellen Prozesses.
// Unterstützt nur "~" (nicht "~user").
func expandUserPath(path string) string {
    if path == "" {
        return path
    }
    if strings.HasPrefix(path, "~") {
        // Nur direktes ~ oder ~/...
        if len(path) == 1 || path[1] == '/' || path[1] == '\\' {
            // Plattformübergreifend: Unter Windows erlauben wir "%USERPROFILE%" Auflösung,
            // sodass die Konfiguration mit "~/..." portabel bleibt.
            if runtime.GOOS == "windows" {
                // Ersetze ~ durch %USERPROFILE% und lasse os.ExpandEnv später auflösen
                // Entferne führenden ~ und eventuellen Separator
                tail := strings.TrimPrefix(path, "~")
                tail = strings.TrimPrefix(tail, "/")
                tail = strings.TrimPrefix(tail, "\\")
                return filepath.Join("%USERPROFILE%", tail)
            }
            if home, err := os.UserHomeDir(); err == nil && home != "" {
                // Ersetze nur das führende ~
                return filepath.Join(home, strings.TrimPrefix(path, "~"))
            }
        }
    }
    return path
}
