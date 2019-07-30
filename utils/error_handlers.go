package utils

import (
	"log"
	"os"
)

func HandleErrEXIT(err error, text string) {
	if err != nil {
		log.Printf("ERROR: %v at %v", err, text)
		os.Exit(1)
	}
}

func HandleErrPANIC(err error, text string) {
	if err != nil {
		log.Printf("ERROR: %v at %v", err, text)
		panic(err)
	}
}

func HandleErrLog(err error, text string) {
	if err != nil {
		log.Printf("ERROR: %v at %v", err, text)
	}
}
