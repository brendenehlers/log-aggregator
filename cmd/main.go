package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	// TODO point this back to /var when running in k8s
	/*
		const podLogDir string = "var/log/pods/"
		podName := "default_counter_1544cea7-4641-4a3b-9ccb-702d941295b3"
		logName := "0.log"
	*/

	fmt.Println("hello")

	f, err := os.Open("var/log/pods/default_counter_1544cea7-4641-4a3b-9ccb-702d941295b3/count/0.log")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		fmt.Println("next line:")
		fmt.Println(s.Text())
	}
}

type cri struct {
	content   string
	stream    string
	flags     rune
	timestamp string
}
