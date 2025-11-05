package music

import (
    "errors"
    "os"
    "path/filepath"

    "github.com/elpatron68/dstask-ui/internal/config"
    "gopkg.in/yaml.v3"
)

type TaskMusic struct {
    Type    string  `yaml:"type"`              // "radio" | "folder"
    Name    string  `yaml:"name"`
    URL     string  `yaml:"url,omitempty"`     // for radio
    Path    string  `yaml:"path,omitempty"`    // for folder
    Volume  float64 `yaml:"volume,omitempty"`
    Muted   bool    `yaml:"muted,omitempty"`
    Shuffle bool    `yaml:"shuffle,omitempty"`
}

type Map struct {
    Version       int                  `yaml:"version"`
    DefaultVolume float64              `yaml:"defaultVolume,omitempty"`
    Tasks         map[string]TaskMusic `yaml:"tasks"`
}

func DefaultMap() *Map {
    return &Map{Version: 1, DefaultVolume: 0.8, Tasks: map[string]TaskMusic{}}
}

// LoadForUser loads ~/.dstask/music-map.yaml for the given username.
func LoadForUser(cfg *config.Config, username string) (*Map, string, error) {
    home, ok := config.ResolveHomeForUsername(cfg, username)
    if !ok || home == "" {
        if h, err := os.UserHomeDir(); err == nil && h != "" {
            home = h
        }
    }
    if home == "" {
        return nil, "", errors.New("home directory could not be determined")
    }
    base := home
    if filepath.Base(base) != ".dstask" {
        base = filepath.Join(home, ".dstask")
    }
    path := filepath.Join(base, "music-map.yaml")
    data, err := os.ReadFile(path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            m := DefaultMap()
            return m, path, nil
        }
        return nil, path, err
    }
    m := DefaultMap()
    if err := yaml.Unmarshal(data, m); err != nil {
        return nil, path, err
    }
    if m.Tasks == nil {
        m.Tasks = map[string]TaskMusic{}
    }
    if m.Version == 0 {
        m.Version = 1
    }
    return m, path, nil
}

// SaveForUser writes the map to ~/.dstask/music-map.yaml for the user.
func SaveForUser(cfg *config.Config, username string, m *Map) (string, error) {
    home, ok := config.ResolveHomeForUsername(cfg, username)
    if !ok || home == "" {
        if h, err := os.UserHomeDir(); err == nil && h != "" {
            home = h
        }
    }
    if home == "" {
        return "", errors.New("home directory could not be determined")
    }
    base := home
    if filepath.Base(base) != ".dstask" {
        base = filepath.Join(home, ".dstask")
    }
    if err := os.MkdirAll(base, 0755); err != nil {
        return "", err
    }
    path := filepath.Join(base, "music-map.yaml")
    out, err := yaml.Marshal(m)
    if err != nil {
        return path, err
    }
    if err := os.WriteFile(path, out, 0644); err != nil {
        return path, err
    }
    return path, nil
}


