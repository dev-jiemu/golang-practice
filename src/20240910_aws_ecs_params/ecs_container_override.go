package main

import (
	"fmt"
	"os"
)

/*
	ecs container param test code

	"ContainerOverrides": [
		{
		  "Name": "", // container-name
		  "Command": ["param1 value", "param2 value"]
		}
	]
*/

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: ecs-container-params: <param1> <param2>")
		os.Exit(1)
	}

	param1 := os.Args[1]
	param2 := os.Args[2]

	fmt.Printf("Argument 1: %s\n", param1)
	fmt.Printf("Argument 2: %s\n", param2)
}
