package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
	"context"
)

func main() {
	fmt.Println("hello")

	base := "var/log/pods"
	d, err := os.Open(base)
	if err != nil { panic(err) }
	defer d.Close()

	// get pod refs /var/log/pods
	pods, err := d.ReadDir(-1)
	if err != nil { panic(err) }
	podNames := make([]string, 0, len(pods))
	for _, pod := range pods {
		podNames = append(podNames, pod.Name())
	}

	ctx := context.Background()
	for _, name := range podNames {
		// var/log/pods/<pod ref>/<container-name>/0.log
		podDir := base + "/" + name
		pd, err := os.Open(podDir)
		if err != nil { panic(err) }

		// get containers in the pod
		cs, err := pd.ReadDir(-1)
		if err != nil { panic(err) }
		for _, c := range cs {
			// read log file of container
			// TODO put this section in a goroutine and read everything back via a channel
			log := podDir + "/" + c.Name() + "/0.log"
			err = infiniteReadFile(ctx, log)
			if err != nil { panic(err) }
		}
	}
}

func infiniteReadFile(ctx context.Context, filename string) (error) {
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
