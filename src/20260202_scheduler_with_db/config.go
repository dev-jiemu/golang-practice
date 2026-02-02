package main

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	ScheduleConfig ScheduleConfig
	DbConfig       DbConfig
}

type ScheduleConfig struct {
	ProcessingCount int `envconfig:"PROCESSING_COUNT"` // worker 개수
	PendingCount    int `envconfig:"PENDING_COUNT"`    // 여분의 요청 개수

	MaxDedicatedUsers     int     `envconfig:"MAX_DEDICATED_USERS" default:"3"`        // 선점할 User 수 (ex. 3)
	DedicatedQuotaPercent float64 `envconfig:"DEDICATED_QUOTA_PERCENT" default:"0.25"` // 선점 영역 비율 (예: 0.25 = 25%)
	StatRefreshInterval   int     `envconfig:"STAT_REFRESH_INTERVAL" default:"5"`      // 통계 갱신 주기 (RDS, 단위: second)
}

type DbConfig struct {
	Host     string `envconfig:"DB_HOST" default:"localhost"`
	Port     int    `envconfig:"DB_PORT" default:"3306"`
	User     string `envconfig:"DB_USER"`
	Password string `envconfig:"DB_PASSWORD"`
	Database string `envconfig:"DB_DATABASE"`
}

func InitConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading .env file")
	}

	config := &Config{}

	err = envconfig.Process("", config)
	if err != nil {
		return nil, err
	}

	err = envconfig.Process("DB", config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
