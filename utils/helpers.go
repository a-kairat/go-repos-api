package utils

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
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

// CheckLevel checks depth level in received query parameter.
// If no level provided sets it to 1. If level is greater then 5 or it's a `max`
// sets it to 5 (max depth level). If invalid query parameter received returns error.
func CheckLevel(level string) (string, error) {
	if level == "" {
		level = "1"
	}

	if level == "max" {
		level = "5"
	}

	rLevel, lErr := StrToInt(level)
	if lErr != nil {
		return "", fmt.Errorf("Invalid level")
	}

	if rLevel > 5 {
		level = "5"
	}

	return level, nil
}

func Gzip(w io.Writer, data []byte) error {
	gw, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
	defer gw.Close()
	gw.Write(data)
	return err
}

func Ungzip(w io.Writer, data []byte) error {
	gr, err := gzip.NewReader(bytes.NewBuffer(data))
	defer gr.Close()
	data, err = ioutil.ReadAll(gr)
	if err != nil {
		return err
	}
	w.Write(data)
	return nil
}
