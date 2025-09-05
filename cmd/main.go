package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
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
		criLog := parseLog(s.Text())
		fmt.Println(criLog)
	}
}

type criLog struct {
	Content   string
	Stream    string
	Flags     string
	Timestamp string
}

func parseLog(log string) (criLog) {
	// 2025-09-05T01:25:22.667941074Z stdout F 132: Fri Sep  5 01:25:22 UTC 2025
	strs := strings.SplitN(log, " ", 4)
	return criLog {
		Content: strs[3],
		Stream: strs[1],
		Flags: strs[2],
		Timestamp: strs[0],
	}
}
