package main

import (
	"fmt"
	"time"
)

func main() {
	timestampMilliseconds := int64(1693245600000)
	timestampSeconds := timestampMilliseconds / 1000
	t := time.Unix(timestampSeconds, 0)
	formattedTime := t.Format("2006-01-02 15:04:05") // 원하는 형식으로 포맷팅
	fmt.Println(formattedTime)
}
