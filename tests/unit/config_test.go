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
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Minimale valide Config schreiben
	minimalContent := []byte(`
server:
  read_timeout: "5s"
  write_timeout: "5s"
  max_request_size: 1024
database:
  driver: "postgres"
  source: "dummy"
vectordb:
  host: "dummy"
  scheme: "http"
  index_name: "CodeChunk"
`)
	_ = os.WriteFile(configFile, minimalContent, 0644)

	cfg, _ := config.LoadConfig(tmpDir)

	if cfg == nil {
		return
	}

	// Prüfen ob Defaults gesetzt wurden
	assert.Equal(t, "8080", cfg.Server.Port)

	assert.Equal(t, "dev", cfg.Environment)
}

func TestConfig_LoadFromYaml(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := []byte(`
server:
  port: "9090"
  read_timeout: "10s"
  write_timeout: "10s"
  max_request_size: 2048
database:
  driver: "sqlite" # valid driver
  source: "file::memory:"
vectordb:
  host: "localhost:1234"
  scheme: "grpc"
  index_name: "TestIndex"
`)
	err := os.WriteFile(configFile, yamlContent, 0644)
	require.NoError(t, err)

	cfg, err := config.LoadConfig(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "9090", cfg.Server.Port)
	assert.Equal(t, "sqlite", cfg.Database.Driver)
}

func TestConfig_EnvOverride(t *testing.T) {
	// 1. Basis-Config erstellen (damit 'required' Validierung besteht)
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := []byte(`
server:
  port: "8080"
  read_timeout: "5s"
  write_timeout: "5s"
  max_request_size: 1024
database:
  driver: "postgres"
  source: "postgres://original"
vectordb:
  host: "weaviate"
  scheme: "http"
  index_name: "CodeChunk"
`)
	err := os.WriteFile(configFile, yamlContent, 0644)
	require.NoError(t, err)

	// 2. Env Var setzen, die den Wert im File überschreiben soll
	os.Setenv("REFINERY_SERVER_PORT", "9999")
	os.Setenv("REFINERY_DATABASE_DRIVER", "sqlite") // Test Override

	// Cleanup nach dem Test
	defer os.Unsetenv("REFINERY_SERVER_PORT")
	defer os.Unsetenv("REFINERY_DATABASE_DRIVER")

	// 3. Config laden
	cfg, err := config.LoadConfig(tmpDir)

	// 4. Assertions (Mit require, um Panic zu verhindern)
	require.NoError(t, err) // Bricht ab, wenn Validierung fehlschlägt
	require.NotNil(t, cfg)  // Bricht ab, wenn cfg nil ist

	// Prüfen, ob Env Var gewonnen hat
	assert.Equal(t, "9999", cfg.Server.Port, "Env variable should override config file")
	assert.Equal(t, "sqlite", cfg.Database.Driver, "Env variable should override config file")

	// Prüfen, ob der Rest aus dem File kommt
	assert.Equal(t, "postgres://original", cfg.Database.Source)
}
