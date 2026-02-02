package main

import (
	"context"
	"example/common"
	"time"

	"github.com/labstack/gommon/log"
)

var SchedulerConfig *Config

func main() {
	log.SetLevel(log.DEBUG)

	var err error
	SchedulerConfig, err = InitConfig()
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("Setting Config", SchedulerConfig)

	dbConfig := &common.DBConfig{
		Host:            SchedulerConfig.DbConfig.Host,
		Port:            SchedulerConfig.DbConfig.Port,
		Username:        SchedulerConfig.DbConfig.User,
		Password:        SchedulerConfig.DbConfig.Password,
		Database:        SchedulerConfig.DbConfig.Database,
		MaxIdleConns:    5,
		MaxOpenConns:    5,
		ConnMaxLifetime: 3 * time.Minute, // 커넥션 재사용 수명
	}

	err = common.Init(dbConfig)
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("Setting DB", dbConfig)

	queue := NewQueueRepo()
	counts, err := queue.GetCountInfo(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, info := range counts {
		log.Debugf("info : %+v", info)
	}
}
