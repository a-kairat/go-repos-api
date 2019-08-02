package utils

import (
	"log"
	"os"
)

// HandleErrEXIT logs error and text. Exits process.
func HandleErrEXIT(err error, text string) {
	if err != nil {
		log.Printf("ERROR: %v at %v", err, text)
		os.Exit(1)
	}
}

// HandleErrPANIC logs error and text. Calls `panic` on error
func HandleErrPANIC(err error, text string) {
	if err != nil {
		log.Printf("ERROR: %v at %v", err, text)
		panic(err)
	}
}

// HandleErrLog logs error and text. Does not exit or panic.
func HandleErrLog(err error, text string) {
	if err != nil {
		log.Printf("ERROR: %v at %v", err, text)
	}
}
