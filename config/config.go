package config

import (
	"encoding/json"
	"os"
	"time"
)

type AppConfig struct {
	DefaultPort             string   `json:"default_port"`
	MaxClients              int      `json:"max_clients"`
	TimeFormat              string   `json:"time_format"`
	AdminSecret             string   `json:"admin_secret"`
	IdleTimeoutSeconds      int      `json:"idle_timeout_seconds"`
	WarningThresholdSeconds int      `json:"warning_threshold_seconds"`
	CertFile                string   `json:"cert_file"`
	KeyFile                 string   `json:"key_file"`
	BannedWords             []string `json:"banned_words"`

	// Parsed execution window variables
	IdleTimeout      time.Duration
	WarningThreshold time.Duration
}

const LinuxLogo = "Welcome to Secure-TCP-Chat!\n" +
	"         _nnnn_\n" +
	"        dGGGGMMb\n" +
	"       @p~qp~~qMb\n" +
	"       M|@||@) M|\n" +
	"       @,----.JM|\n" +
	"      JS^\\__/  qKL\n" +
	"     dZP        qKRb\n" +
	"    dZP          qKKb\n" +
	"   fZP            SMMb\n" +
	"   HZM            MMMM\n" +
	"   FqM            MMMM\n" +
	" __| \".        |\\dS\"qML\n" +
	" |    `.       | `' \\Zq\n" +
	"_)      \\.___.,|     .'\n" +
	"\\____   )MMMMMP|   .'\n" +
	"     `-'       `--'\n" +
	"[ENTER YOUR NAME]: "

func LoadConfig(path string) (*AppConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg AppConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	cfg.IdleTimeout = time.Duration(cfg.IdleTimeoutSeconds) * time.Second
	cfg.WarningThreshold = time.Duration(cfg.WarningThresholdSeconds) * time.Second
	return &cfg, nil
}
