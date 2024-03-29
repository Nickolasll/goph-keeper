// Package config содержит конфиг клиента
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const dbFileMode = 0600
const configName = "config.json"

// Config - Конфигурация клиента
type Config struct {
	// DBFileMode - Режим чтения файла базы данных
	DBFileMode uint32
	// ServerBasePath - базовый url сервера
	ServerBasePath string
	// DBFilePath - Путь до файла базы данных
	DBFilePath string `json:"db_file_path"`
	// ClientTimeout - Таймаут запроса клиента до сервера
	ClientTimeout time.Duration `json:"db_client_timeout"`
	// ServerURL - URL сервера
	ServerURL string `json:"server_url"`
}

// New - Возвращает инстанс конфигурации сервера из файла
func New(root string) (*Config, error) {
	cfg := Config{
		DBFileMode:     dbFileMode,
		ClientTimeout:  time.Duration(30) * time.Second, //nolint: gomnd
		ServerBasePath: "api/v1/",
		DBFilePath:     "user.db",
	}

	configPath := filepath.Join(root, configName)

	data, err := os.ReadFile(configPath) //nolint: gosec
	if err != nil {
		return &cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return &cfg, err
	}

	return &cfg, nil
}
