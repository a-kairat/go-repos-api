package utils

import (
	"fmt"
	"strconv"
)

// IntToStr converts type `int` to type `string`
func IntToStr(n int) string {
	return strconv.Itoa(n)
}

// StrToInt converts type `string` to type `int`
func StrToInt(str string) (int, error) {
	n, err := strconv.Atoi(str)
	if err != nil {
		return -1, fmt.Errorf("Error converting str: %v to type `int`", str)
	}
	return n, nil
}

func CheckLevel(level string) (string, error) {
	if level == "" {
		level = "1"
	}

	if level == "max" {
		level = "9"
	}

	rLevel, lErr := StrToInt(level)
	if lErr != nil {
		return "", fmt.Errorf("Invalid level")
	}

	if rLevel > 9 {
		level = "9"
	}

	return level, nil
}
