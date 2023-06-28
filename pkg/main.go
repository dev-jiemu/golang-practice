package main

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var randomString = generateRandomSentence(15)

const (
	letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func main() {
	// Echo 인스턴스 생성
	e := echo.New()

	// API 엔드포인트 등록
	e.GET("/api", handleAPI)

	// 서버 시작
	e.Start(":8090")
}

func handleAPI(c echo.Context) error {
	var err error

	now := time.Now()
	date := now.Format("2006-01-02") // 날짜 형식 지정

	// 로그 파일 생성
	fileName := fmt.Sprintf("/data/log/%s.log", date)
	//fileName := fmt.Sprintf("/Users/jiyekim/workspace/golang_efs_tester/log/%s.log", date)

	// 디렉토리 없으면 생성
	dirPath := filepath.Dir(fileName)
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	// 파일열기: 없으면 생성
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// 로그 기록
	logger := log.New(file, "", log.LstdFlags)
	logger.Printf("[%s] String : %s", now.String(), randomString)
	fmt.Printf("[%s] String : %s", now.String(), randomString)

	return c.String(http.StatusOK, "")

}

func generateRandomSentence(maxLength int) string {
	sentence := make([]byte, maxLength)
	for i := 0; i < maxLength; i++ {
		sentence[i] = letters[rand.Intn(len(letters))]
	}
	return string(sentence)
}
