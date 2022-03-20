package main

import (
	"log"
	"os"
)

func checkErr(err error) {
	if err != nil {
		log.Printf("ERROR: %s", err.Error())
	}
}

func readFile(filename string) (string, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func writeFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), os.ModePerm)
}
