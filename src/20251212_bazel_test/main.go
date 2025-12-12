package main

import (
	"fmt"

	"github.com/streamer45/silero-vad-go/speech"
)

func main() {
	config := &speech.DetectorConfig{
		SampleRate: 16000,
	}

	fmt.Println(config.SampleRate)
}
