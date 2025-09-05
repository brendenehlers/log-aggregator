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
	ctx := context.Background()
	outChan := make(chan criLog, 100)
	err := createReaderRoutines(ctx, base, outChan)
	if err != nil { panic(err) }

	// wait for results
	for {
		select {
		case log := <-outChan:
			fmt.Println(log)
		case <-ctx.Done():
			panic(ctx.Err())
		}
	}
}

func createReaderRoutines(ctx context.Context, base string, out chan<- criLog) (error) {
	d, err := os.Open(base)
	if err != nil { return err }
	defer d.Close()

	// get pod refs /var/log/pods
	podNames, err := d.Readdirnames(-1)
	if err != nil { return err }

	for _, name := range podNames {
		// TODO remove this--temp to skip this pod's logs
		if strings.Contains(name, "log-pod") { continue }
		// var/log/pods/<pod ref>/<container-name>/<instance#>.log
		podDir := base + "/" + name
		pd, err := os.Open(podDir)
		if err != nil { return err }
		defer pd.Close()

		// get containers in the pod
		cs, err := pd.Readdirnames(-1)
		if err != nil { return err }
		for _, c := range cs {
			// read log file of container
			cDir, err := os.Open(podDir + "/" + c)
			if err != nil { return err }
			defer cDir.Close()

			instances, err := cDir.Readdirnames(-1)
			if len(instances) < 1 { continue }
			log := podDir + "/" + c + "/" + instances[0] // TODO verify k8s will only create 1 file in this dir
			go func() {
				err = infiniteReadFile(ctx, log, out)
				if err != nil { panic(err) }
			}()
		}
	}
	return nil
}

func infiniteReadFile(ctx context.Context, filename string, out chan<- criLog) (error) {
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
			out <- criLog
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
