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
	podNames, err := d.Readdirnames(-1)
	if err != nil { panic(err) }

	ctx := context.Background()
	for _, name := range podNames {
		// var/log/pods/<pod ref>/<container-name>/0.log
		podDir := base + "/" + name
		pd, err := os.Open(podDir)
		if err != nil { panic(err) }
		defer pd.Close()

		// get containers in the pod
		cs, err := pd.Readdirnames(-1)
		if err != nil { panic(err) }
		for _, c := range cs {
			// read log file of container
			cDir, err := os.Open(podDir + "/" + c)
			if err != nil { panic(err) }
			defer cDir.Close()

			// TODO put this section in a goroutine and read everything back via a channel
			instances, err := cDir.Readdirnames(-1)
			log := podDir + "/" + c + "/" + instances[0] // TODO verify k8s will only create 1 file in this dir
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

	for {
		s := bufio.NewScanner(f)
		// read lines from file and prints the parsed criLog
		for s.Scan() {
			txt := s.Text()
			if txt == "" { continue }
			criLog := parseLog(txt)
			fmt.Println(criLog)
		}
		// error out if err != EOF
		if err := s.Err(); err != nil {
			return err
		}
		// listen for context clues
		select {
		case <- ctx.Done():
			return ctx.Err()
		default:
			// wait for more logs
			time.Sleep(1 * time.Second)
		}
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
	strs := strings.SplitN(log, " ", 4)
	return criLog {
		Content: strs[3],
		Stream: strs[1],
		Flags: strs[2],
		Timestamp: strs[0],
	}
}
