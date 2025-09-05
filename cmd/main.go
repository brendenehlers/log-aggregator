package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	// TODO point this back to /var when running in k8s
	/*
		const podLogDir string = "var/log/pods/"
		podName := "default_counter_1544cea7-4641-4a3b-9ccb-702d941295b3"
		logName := "0.log"
	*/

	fmt.Println("hello")

	// this one for kind
	// fp := "var/log/pods/default_counter_9aad6fe0-3cb3-451c-80d8-b6d107cc1fd2/count/0.log"
	// this one for local
	fp := "var/log/pods/default_counter_1544cea7-4641-4a3b-9ccb-702d941295b3/count/0.log"
	infiniteReadFile(fp)
}

func infiniteReadFile(filename string) (error) {

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	for true {
		s := bufio.NewScanner(f)
		// read lines from file and prints the parsed criLog
		for s.Scan() {
			txt := s.Text()
			if txt == "" { continue }
			criLog := parseLog(txt)
			fmt.Println(criLog)
		}
		// panic if err != EOF
		if err := s.Err(); err != nil {
			return err
		}
		// wait for more logs
		time.Sleep(1 * time.Second)
	}
	return nil
}

type criLog struct {
	Content   string
	Stream    string
	Flags     string
	Timestamp string
}

func parseLog(log string) (criLog) {
	// 2025-09-05T01:25:22.667941074Z stdout F 132: Fri Sep  5 01:25:22 UTC 2025
	fmt.Println(log)
	strs := strings.SplitN(log, " ", 4)
	return criLog {
		Content: strs[3],
		Stream: strs[1],
		Flags: strs[2],
		Timestamp: strs[0],
	}
}
