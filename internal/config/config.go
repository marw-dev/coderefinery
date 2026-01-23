package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Config struct {
	Environment 	string         		`mapstructure:"environment" validate:"required,oneof=dev staging production"`
	Server      	ServerConfig   		`mapstructure:"server" validate:"required"`
	Indexer     	IndexerConfig 	 	`mapstructure:"indexer" validate:"required"`
	Database    	DatabaseConfig 		`mapstructure:"database" validate:"required"`
	Auth        	AuthConfig     		`mapstructure:"auth" validate:"required"`
	LLM         	LLMConfig     		`mapstructure:"llm" validate:"required"`
	Search      	SearchConfig   		`mapstructure:"search"`
	Observability 	ObservabilityConfig `mapstructure:"observability"`
	Cache			CacheConfig 		`mapstructure:"cache"`
}

type ServerConfig struct {
	Port           string        `mapstructure:"port" validate:"required,numeric"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout" validate:"required"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout" validate:"required"`
	MaxRequestSize int64         `mapstructure:"max_request_size" validate:"min=1024"`
	EnableCORS     bool          `mapstructure:"enable_cors"`
	AllowedOrigins []string      `mapstructure:"allowed_origins"`
}


type IndexerConfig struct {
	SupportedExts map[string]string `mapstructure:"supported_extensions"`

	ExcludePaths  []string      `mapstructure:"exclude_paths"`
	MinChunkSize  int           `mapstructure:"min_chunk_size" validate:"min=10"`
	MaxChunkSize  int           `mapstructure:"max_chunk_size" validate:"gtfield=MinChunkSize"`
	BatchSize     int           `mapstructure:"batch_size" validate:"min=1"`
	WatchDebounce time.Duration `mapstructure:"watch_debounce"`
}

type DatabaseConfig struct {
	Driver			string 	 	  `mapstructure:"driver" validate:"required,oneof=sqlite postgres"`
	Source			string 		  `mapstructure:"source" validate:"required"`
	MaxOpenConns    int           `mapstructure:"max_open_conns" validate:"min=1"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns" validate:"min=1"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime" validate:"min=1m"`
}

type AuthConfig struct {
	JWTSecret string        `mapstructure:"jwt_secret" validate:"required,min=32"`
	JWTExpiry time.Duration `mapstructure:"jwt_expiry" validate:"min=1m"`
}

type LLMConfig struct {
	Service        string        `mapstructure:"service" validate:"required,oneof=ollama openai"`
	Host           string        `mapstructure:"host" validate:"required,url"`
	EmbeddingModel string        `mapstructure:"embedding_model" validate:"required"`
	Timeout        time.Duration `mapstructure:"timeout"`
}

type SearchConfig struct {
	DefaultLimit int     `mapstructure:"default_limit"`
	MaxLimit     int     `mapstructure:"max_limit"`
	MinScore     float64 `mapstructure:"min_score"`
}

type ObservabilityConfig struct {
	Logging LoggingConfig `mapstructure:"logging"`
	Metrics MetricsConfig `mapstructure:"metrics"`
	Tracing TracingConfig `mapstructure:"tracing"`
}

type TracingConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	Provider     string  `mapstructure:"provider"`      // z.B. "jaeger"
	Endpoint     string  `mapstructure:"endpoint"`      // z.B. "http://localhost:14268/api/traces"
	SamplingRate float64 `mapstructure:"sampling_rate"` // 0.0 bis 1.0
	ServiceName  string  `mapstructure:"service_name"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"` // z.B. "/metrics"
	Port    string `mapstructure:"port"` // Optional: Eigener Port für Metriken (oder leer lassen für gleichen Port)
}

type LoggingConfig struct {
	Level  string `mapstructure:"level" validate:"required,oneof=debug info warn error"`
	Format string `mapstructure:"format" validate:"required,oneof=json console"`
}

type CacheConfig struct {
    Enabled   bool          `mapstructure:"enabled"`
    RedisURL  string        `mapstructure:"redis_url"` // z.B. "redis://localhost:6379/0"
    TTL       time.Duration `mapstructure:"ttl"`       // Wie lange speichern? z.B. 1h
}

// NewDefault erstellt Defaults
func NewDefault() *Config {
	return &Config{
		Environment: "dev",
		Server: ServerConfig{
			Port:           "8080",
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			MaxRequestSize: 50 << 20,
			EnableCORS:     true,
			AllowedOrigins: []string{"*"},
		},
		LLM: LLMConfig{
			Service:        "ollama",
			Host:           "http://localhost:11434",
			EmbeddingModel: "nomic-embed-text",
			Timeout:        60 * time.Second,
		},
		Indexer: IndexerConfig{
			SupportedExts: map[string]string{
				"go": "go", "py": "python", "js": "javascript", "ts": "typescript",
				"java": "java", "rs": "rust", "cpp": "cpp", "c": "c",
			},
			ExcludePaths: []string{
				"node_modules", "vendor", ".git", "__pycache__", "dist", "build",
			},
			MinChunkSize:  50,
			MaxChunkSize:  1500,
			BatchSize:     10,
			WatchDebounce: 2 * time.Second,
		},
		Database: DatabaseConfig{
			Driver: "postgres", // Empfohlen für pgvector
			Source: "postgres://refinery:secret@localhost:5434/coderefinery?sslmode=disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 15 * time.Minute,
		},
		Auth: AuthConfig{
			JWTSecret: "change-me-to-a-very-secure-secret-key-please",
			JWTExpiry: 24 * time.Hour,
		},
		Search: SearchConfig{
			DefaultLimit: 10,
			MaxLimit:     50,
			MinScore:     0.5,
		},
		Observability: ObservabilityConfig{
			Logging: LoggingConfig{
				Level:  "info",
				Format: "console", // In Dev lieber console, in Prod json
			},
			Metrics: MetricsConfig{
				Enabled: true,
				Path:    "/metrics",
				Port:    "", // Leer = läuft auf dem Haupt-Server-Port (8080)
			},
			Tracing: TracingConfig{
				Enabled:      false,
				Provider:     "jaeger",
				Endpoint:     "http://localhost:14268/api/traces",
				SamplingRate: 1.0,
				ServiceName:  "coderefinery",
			},
		},
		Cache: CacheConfig{
            Enabled:  true,
            RedisURL: "redis://localhost:6379/0",
            TTL:      1 * time.Hour,
        },
	}
}

// LoadConfig lädt Konfiguration
func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	// Defaults setzen
	defaults := NewDefault()
	v.SetDefault("environment", defaults.Environment)
	v.SetDefault("server", defaults.Server)
	v.SetDefault("indexer", defaults.Indexer)
	v.SetDefault("database", defaults.Database)

	v.SetDefault("database.max_open_conns", defaults.Database.MaxOpenConns)
    v.SetDefault("database.max_idle_conns", defaults.Database.MaxIdleConns)
    v.SetDefault("database.conn_max_lifetime", defaults.Database.ConnMaxLifetime)

	v.SetDefault("auth", defaults.Auth)
	v.SetDefault("llm", defaults.LLM)
	v.SetDefault("search", defaults.Search)

	// Logging
	v.SetDefault("observability.logging.level", defaults.Observability.Logging.Level)
	v.SetDefault("observability.logging.format", defaults.Observability.Logging.Format)

	// Metrics
	v.SetDefault("observability.metrics.enabled", defaults.Observability.Metrics.Enabled)
	v.SetDefault("observability.metrics.path", defaults.Observability.Metrics.Path)
	v.SetDefault("observability.metrics.port", defaults.Observability.Metrics.Port)

	// Tracing
	v.SetDefault("observability.tracing.enabled", defaults.Observability.Tracing.Enabled)
	v.SetDefault("observability.tracing.provider", defaults.Observability.Tracing.Provider)
	v.SetDefault("observability.tracing.endpoint", defaults.Observability.Tracing.Endpoint)
	v.SetDefault("observability.tracing.sampling_rate", defaults.Observability.Tracing.SamplingRate)
	v.SetDefault("observability.tracing.service_name", defaults.Observability.Tracing.ServiceName)

	v.SetDefault("cache", defaults.Cache)

	// Config Datei suchen
	v.AddConfigPath(".")
	v.AddConfigPath(path)
	v.SetConfigName("config")

	// KORREKTUR: YAML erzwingen
	v.SetConfigType("yaml")

	// Environment Variables
	v.SetEnvPrefix("REFINERY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Datei lesen
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Validation
	validate := validator.New()
	if err := validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}
