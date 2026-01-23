package unit

import (
	"os"
	"path/filepath"
	"testing"

	"coderefinery/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := config.NewDefault()

	// Observability Defaults prüfen
	assert.Equal(t, "info", cfg.Observability.Logging.Level)
	assert.Equal(t, "console", cfg.Observability.Logging.Format)

	assert.True(t, cfg.Observability.Metrics.Enabled)
	assert.Equal(t, "/metrics", cfg.Observability.Metrics.Path)

	assert.False(t, cfg.Observability.Tracing.Enabled)
}

func TestConfig_LoadFromYaml(t *testing.T) {
	// 1. Temporäres Verzeichnis
	tmpDir, err := os.MkdirTemp("", "refinery_config_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 2. YAML Inhalt definieren (MIT GÜLTIGEN WERTEN!)
	yamlContent := `
environment: production
server:
  port: "9090"
  read_timeout: 10s
  write_timeout: 10s
  max_request_size: 1048576 # > 1024 (min validator)
database:
  driver: postgres
  source: dummy
llm:
  service: ollama
  host: http://localhost:11434
  embedding_model: test-model
indexer:
  supported_extensions:
    go: go
  # Validierung beachten!
  min_chunk_size: 50
  max_chunk_size: 1500
  batch_size: 10
auth:
  # Validierung: min=32 Zeichen
  jwt_secret: "dies-ist-ein-sehr-langes-geheimes-secret-fuer-den-test"
  jwt_expiry: 24h
observability:
  logging:
    level: debug
    format: json
  metrics:
    enabled: false
  tracing:
    enabled: true
    provider: otlp
    endpoint: localhost:4318
    service_name: test-service
`
	// 3. Datei schreiben
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// 4. Config laden
	cfg, err := config.LoadConfig(tmpDir)

	// Falls hier Fehler kommen, geben wir sie aus
	if err != nil {
		t.Logf("Config Load Error: %v", err)
	}
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// 5. Assertions
	assert.Equal(t, "production", cfg.Environment)
	assert.Equal(t, "9090", cfg.Server.Port)

	assert.Equal(t, "debug", cfg.Observability.Logging.Level)
	assert.False(t, cfg.Observability.Metrics.Enabled)
	assert.True(t, cfg.Observability.Tracing.Enabled)
	assert.Equal(t, "otlp", cfg.Observability.Tracing.Provider)
	assert.Equal(t, "test-service", cfg.Observability.Tracing.ServiceName)
}

func TestConfig_EnvOverride(t *testing.T) {
	// Separate Testfunktion für Environment-Variablen (sauberer)
	os.Setenv("REFINERY_OBSERVABILITY_TRACING_ENABLED", "true")
	defer os.Unsetenv("REFINERY_OBSERVABILITY_TRACING_ENABLED")

	// Lädt Defaults + Env (ignoriert fehlende config.yaml)
	cfg, err := config.LoadConfig(".")
	require.NoError(t, err)

	// Env Variable sollte den Default (false) überschreiben
	assert.True(t, cfg.Observability.Tracing.Enabled)
}
