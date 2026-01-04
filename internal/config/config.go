package config

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	ProjectPath string        `json:"project_path"`
	ServerPort  string        `json:"server_port"`
	Server      ServerConfig  `json:"server"`
	Ollama      OllamaConfig  `json:"ollama"`
	Indexer     IndexerConfig `json:"indexer"`
}

type ServerConfig struct {
	Port            string        `json:"port"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`
	MaxRequestSize  int64         `json:"max_request_size"`
	EnableCORS      bool          `json:"enable_cors"`
}

type OllamaConfig struct {
	BaseURL string        `json:"base_url"`
	Model   string        `json:"model"`
	Timeout time.Duration `json:"timeout"`
}

type IndexerConfig struct {
	SupportedExts    map[string]string `json:"supported_extensions"`
	ExcludePaths     []string          `json:"exclude_paths"`
	MinChunkSize     int               `json:"min_chunk_size"`
	MaxChunkSize     int               `json:"max_chunk_size"`
	BatchSize        int               `json:"batch_size"`
	WatchDebounce    time.Duration     `json:"watch_debounce"`
}

func NewDefault() *Config {
	return &Config{
		ProjectPath: ".",
		ServerPort:  "8080",
		Server: ServerConfig{
			Port:           "8080",
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxRequestSize: 10 << 20, // 10MB
			EnableCORS:     true,
		},
		Ollama: OllamaConfig{
			BaseURL: "http://localhost:11434",
			Model:   "nomic-embed-text",
			Timeout: 30 * time.Second,
		},
		Indexer: IndexerConfig{
			SupportedExts: map[string]string{
				".go":   "go",
				".py":   "python",
				".js":   "javascript",
				".ts":   "typescript",
				".java": "java",
				".rs":   "rust",
				".cpp":  "cpp",
				".c":    "c",
			},
			ExcludePaths: []string{
				"node_modules", "vendor", ".git", "__pycache__",
				"dist", "build", "target", ".venv", "venv",
			},
			MinChunkSize:  50,
			MaxChunkSize:  2000,
			BatchSize:     10,
			WatchDebounce: 2 * time.Second,
		},
	}
}

func (c *Config) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, c)
}