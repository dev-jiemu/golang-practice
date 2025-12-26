package main

import (
	"encoding/json"
	"example/20251224_rtmp_sample/server"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yutopp/go-rtmp"
)

func LoadConfig() error {
	server.WowzaConfig = &server.Config{}

	configFile, err := os.Open("./config.json")
	if err != nil {
		return fmt.Errorf("Error opening config.json file: %s", err)
	}
	defer configFile.Close()

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(server.WowzaConfig)
	if err != nil {
		return fmt.Errorf("Error parsing config.json file: %s", err)
	}

	if server.WowzaConfig.WowzaHost == "" {
		return fmt.Errorf("Wowza_host is required")
	}

	return nil
}

func main() {
	err := LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config.json file: %s", err)
	}

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	log.SetLevel(log.DebugLevel)

	log.Info("========================================")
	log.Info("RTMP Server for wowza forward")
	log.Info("========================================")

	tcpAddr, err := net.ResolveTCPAddr("tcp", ":1935")
	if err != nil {
		log.Panicf("TCP 주소 resolve 실패: %+v", err)
	}
	log.Infof("TCP 주소 설정 완료: %s", tcpAddr.String())

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Panicf("TCP 리스너 생성 실패: %+v", err)
	}
	log.Infof("TCP 리스너 시작: %s", listener.Addr().String())

	connectionCount := 0

	srv := rtmp.NewServer(&rtmp.ServerConfig{
		OnConnect: func(conn net.Conn) (io.ReadWriteCloser, *rtmp.ConnConfig) {
			connectionCount++
			connID := connectionCount

			log.WithFields(log.Fields{
				"connection_id": connID,
				"remote_addr":   conn.RemoteAddr().String(),
				"local_addr":    conn.LocalAddr().String(),
			}).Info("🔌 새로운 클라이언트 연결 시도")

			l := log.WithFields(log.Fields{
				"connection_id": connID,
			})
			l.Logger.SetLevel(log.DebugLevel)

			h := &server.RelayHandler{}

			log.WithFields(log.Fields{
				"connection_id":    connID,
				"bandwidth_window": 6 * 1024 * 1024 / 8,
			}).Debug("연결 설정 완료")

			return conn, &rtmp.ConnConfig{
				Handler: h,

				ControlState: rtmp.StreamControlStateConfig{
					DefaultBandwidthWindowSize: 6 * 1024 * 1024 / 8,
				},

				Logger: l.Logger,
			}
		},
	})

	log.Info("========================================")
	log.Info("클라이언트 연결 대기 중...")

	// 서버 시작 시간 기록
	startTime := time.Now()

	if err := srv.Serve(listener); err != nil {
		log.WithFields(log.Fields{
			"uptime": time.Since(startTime),
			"error":  err,
		}).Panicf("❌ 서버 실행 실패")
	}

}
