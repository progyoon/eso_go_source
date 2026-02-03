package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	Elasticsearch struct {
		Host       string `yaml:"host"`
		Username   string `yaml:"username"`
		Password   string `yaml:"password"`
		IndexName  string `yaml:"index_name"`
		Generation string `yaml:"generation"`
	} `yaml:"elasticsearch"`
	FileDir struct {
		ScanDir     string `yaml:"scan_dir"`
		SQLiteDBDir string `yaml:"sqlite_dir"` // 예: "/remote/du"
	} `yaml:"file_dir"`
	Logging struct {
		LogPrefix        string `yaml:"log_prefix"`        // 로그 파일 접두사 (예: "fetch_xml_files")
		RetentionDays    int    `yaml:"retention_days"`    // 로그 보존 기간 (일)
		LogLevel         string `yaml:"log_level"`         // 로그 레벨 (DEBUG, INFO, WARN, ERROR)
		LogDir           string `yaml:"log_dir"`           // 로그 파일이 저장될 디렉토리 (예: "/remote/logs")
		CollectionPeriod int    `yaml:"collection_period"` //수집 주기 (분) 5,15,60
	} `yaml:"logging"`
	Worker struct {
		OpenFileWorkerCount int `yaml:"open_file_worker_count"`
	} `yaml:"worker"`
}

func LoadConfig(configPath string) (*Config, error) {
	b, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if err := os.MkdirAll(cfg.Logging.LogDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("log dir: %w", err)
	}
	return &cfg, nil
}
