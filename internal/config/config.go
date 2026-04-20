package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	BotToken          string
	OpenAIAPIKey      string
	DBDSN             string
	VMRemoteWriteURL  string
	TZ                string
	AllowedTelegramID int64
	HealthzPort       int
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("TZ", "Europe/Amsterdam")
	v.SetDefault("HEALTHZ_PORT", 8080)

	cfg := &Config{
		BotToken:          v.GetString("BOT_TOKEN"),
		OpenAIAPIKey:      v.GetString("OPENAI_API_KEY"),
		DBDSN:             v.GetString("DB_DSN"),
		VMRemoteWriteURL:  v.GetString("VM_REMOTE_WRITE_URL"),
		TZ:                v.GetString("TZ"),
		AllowedTelegramID: v.GetInt64("ALLOWED_TELEGRAM_ID"),
		HealthzPort:       v.GetInt("HEALTHZ_PORT"),
	}

	if cfg.BotToken == "" {
		return nil, fmt.Errorf("BOT_TOKEN is required")
	}
	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}

	return cfg, nil
}
