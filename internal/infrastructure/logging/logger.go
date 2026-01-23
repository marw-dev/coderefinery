package logging

import (
	"os"
	"time"

	"coderefinery/internal/config"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init konfiguriert den globalen Zerolog Logger basierend auf der Config
func Init(cfg config.LoggingConfig) {
	// 1. Output Format wählen
	if cfg.Format == "console" {
		// Pretty printing für Entwickler (farbig, lesbar)
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		})
	} else {
		// JSON für Maschinen (default in zerolog)
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	}

	// 2. Log Level setzen
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel // Fallback
	}
	zerolog.SetGlobalLevel(level)

	// Optional: Caller (Dateiname:Zeile) in Logs aufnehmen
	log.Logger = log.With().Caller().Logger()
}
