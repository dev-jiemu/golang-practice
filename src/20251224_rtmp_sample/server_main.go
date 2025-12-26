package main

import (
	"example/20251224_rtmp_sample/server"
	"io"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yutopp/go-rtmp"
)

func main() {
	// 로그 포맷 설정
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	log.SetLevel(log.DebugLevel)

	log.Info("========================================")
	log.Info("RTMP 서버 시작 중...")
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

			h := &server.Handler{}

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
	log.Info("✅ RTMP 서버 준비 완료!")
	log.Info("📡 포트: 1935")
	log.Info("📺 OBS 설정: rtmp://localhost:1935/live")
	log.Info("🔑 스트림 키: 아무거나 (예: test)")
	log.Info("========================================")
	log.Info("클라이언트 연결 대기 중...")
	log.Info("")

	// 서버 시작 시간 기록
	startTime := time.Now()

	if err := srv.Serve(listener); err != nil {
		log.WithFields(log.Fields{
			"uptime": time.Since(startTime),
			"error":  err,
		}).Panicf("❌ 서버 실행 실패")
	}
}
