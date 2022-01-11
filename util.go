package main

import "os"

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func readFile(filename string) string {
	bytes, err := os.ReadFile(filename)
	checkErr(err)
	return string(bytes)
}
