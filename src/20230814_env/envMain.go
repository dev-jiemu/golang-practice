package main

import (
	"fmt"
	"os"
)

func main() {
	var temp = os.Getenv("JIEMU")
	fmt.Printf("Getenv : %s", temp)
}
