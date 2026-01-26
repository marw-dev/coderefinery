package unit

import (
	"coderefinery/internal/config"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Temp dir für Config File
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	t.Run("Load minimal config", func(t *testing.T) {
		minimalContent := []byte(`
server:
  port: "8080"
database:
  source: "postgres://user:pass@localhost:5432/db"
vectordb:
  host: "localhost:8080"
`)
		// G306 Fix: 0600 statt 0644
		_ = os.WriteFile(configFile, minimalContent, 0600)

		cfg, err := config.LoadConfig(tmpDir)
		require.NoError(t, err)
		assert.Equal(t, "8080", cfg.Server.Port)
		assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.Database.Source)
	})

	t.Run("Load full config with env overrides", func(t *testing.T) {
		yamlContent := []byte(`
server:
  port: "9090"
environment: "production"
database:
  source: "postgres://old:pass@localhost:5432/db"
`)
		// G306 Fix: 0600 statt 0644
		err := os.WriteFile(configFile, yamlContent, 0600)
		require.NoError(t, err)

		// Env Vars setzen
		t.Setenv("REF_SERVER_PORT", "10000")
		t.Setenv("REF_DATABASE_SOURCE", "postgres://new:pass@localhost:5432/db")

		cfg, err := config.LoadConfig(tmpDir)
		require.NoError(t, err)

		// Env sollte YAML überschreiben
		assert.Equal(t, "10000", cfg.Server.Port)
		assert.Equal(t, "postgres://new:pass@localhost:5432/db", cfg.Database.Source)
		assert.Equal(t, "production", cfg.Environment)
	})

	t.Run("Validation failure", func(t *testing.T) {
		// Invalid config (missing required DB source)
		yamlContent := []byte(`
server:
  port: "8080"
`)
		// G306 Fix: 0600 statt 0644
		err := os.WriteFile(configFile, yamlContent, 0600)
		require.NoError(t, err)

		_, err = config.LoadConfig(tmpDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Database.Source")
	})
}
