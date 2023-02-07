package main

import "net/smtp"

func main() {
	auth := smtp.PlainAuth("", "", "", "smtp.gmail.com")
	from := ""
	to := []string{""}

	// 메시지 작성
	headerSubject := "Subject: 테스트\r\n"
	headerBlank := "\r\n"
	body := "메일 테스트입니다\r\n"
	msg := []byte(headerSubject + headerBlank + body)

	// 메일 보내기
	err := smtp.SendMail("smtp.gmail.com:587", auth, from, to, msg)
	if err != nil {
		panic(err)
	}
}
