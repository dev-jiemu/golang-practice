package common

import (
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type DBConfig struct {
	Host            string
	Port            int
	Username        string
	Password        string
	Database        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

var instance *sqlx.DB

func (v *DBConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		v.Username,
		v.Password,
		v.Host,
		v.Port,
		v.Database,
	)
}

func Init(cfg *DBConfig) error {
	var err error

	instance, err = sqlx.Open("mysql", cfg.DSN())
	if err != nil {
		return err
	}
	instance.SetMaxOpenConns(cfg.MaxOpenConns)
	instance.SetMaxIdleConns(cfg.MaxIdleConns)
	instance.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	return instance.Ping()
}

func GetDB() *sqlx.DB {
	return instance
}
