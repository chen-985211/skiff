package main

import (
	"log"
	"skiff/internal/platform/desktop"
)

func main() {
	if err := desktop.Run(); err != nil {
		log.Fatal(err)
	}
}
