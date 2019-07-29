package utils

import (
	"fmt"
	"log"
)

func HandleErrPanic(err error, text string) {
	if err != nil {
		fmt.Println(text)
		panic(err)
	}
}

func HandleErrLog(err error, text string) {
	if err != nil {
		log.Println(text)
		// panic(err)
	}
}
