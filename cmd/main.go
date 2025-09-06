package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
	"context"

	grpclog "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/log"
)

func main() {
	fmt.Println("hello")
	ctx := context.Background()

	provider, err := configureProvider(ctx, "grafana:4317")
	if err != nil { panic(err) }
	defer func() { if err := provider.Shutdown(ctx); err != nil { panic(err) } }()
	logger := provider.Logger("com.behlers.log_scraper")

	base := "var/log/pods"
	outChan := make(chan *LogWithMetadata, 100)
	err = createReaderRoutines(ctx, base, outChan)
	if err != nil { panic(err) }

	// wait for results
	for {
		select {
		case log := <-outChan:
			handle(ctx, logger, log)
		case <-ctx.Done():
			if err := ctx.Err(); err != nil { panic(ctx.Err()) } else { break }
		}
	}
}

func configureProvider(ctx context.Context, endpoint string) (*sdklog.LoggerProvider, error) {
	// configure otel logging exporter
	exp, err := grpclog.New(ctx, grpclog.WithEndpoint(endpoint), grpclog.WithInsecure())
	if err != nil { return nil, err }
	processor := sdklog.NewBatchProcessor(exp)
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(processor))
	return provider, nil
}

func handle(ctx context.Context, logger log.Logger, log *LogWithMetadata) {
	fmt.Println(log) // TODO remove this
}

func createReaderRoutines(ctx context.Context, base string, out chan<- *LogWithMetadata) (error) {
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
			metadata := parseMetadata(name, c, strings.Split(instances[0], ".")[0])
			go func() {
				err = infiniteReadFile(ctx, log, out, metadata)
				if err != nil { panic(err) }
			}()
		}
	}
	return nil
}

func parseMetadata(pod, container, instance string) (Metadata) {
	// default_counter_1544cea7-4641-4a3b-9ccb-702d941295b3
	// TODO validate the structure of the dir
	// TODO are underscores allowed in pod names?
	chunks := strings.Split(pod, "_")
	return Metadata {
		Namespace: chunks[0],
		PodName: chunks[1],
		PodId: chunks[2],
		Container: container,
		Instance: instance,
	}
}

func infiniteReadFile(
	ctx context.Context,
	filename string,
	out chan<- *LogWithMetadata,
	metadata Metadata,
) (error) {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	for {
		s := bufio.NewScanner(f)
		// read lines from file and prints the parsed CriLog
		for s.Scan() {
			txt := s.Text()
			if txt == "" { continue }
			criLog, err := parseLog(txt)
			// TODO think about recovering from error here
			if err != nil { return err }
			out <- &LogWithMetadata {
				Log: criLog,
				Metadata: metadata,
			}
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

type LogWithMetadata struct {
	Log CriLog
	Metadata Metadata
}

type Metadata struct {
	Namespace string
	PodName string
	PodId string
	Container string
	Instance string	
}

type CriLog struct {
	Content   string
	Stream    LogStream
	Flags     string
	Timestamp time.Time
}

type LogStream int
const (
	Stdout LogStream = iota
	Stderr
)

func parseLog(log string) (CriLog, error) {
	// 2025-09-05T01:25:22.667941074Z stdout F 132: Fri Sep  5 01:25:22 UTC 2025
	strs := strings.SplitN(log, " ", 4)
	stream, err := parseStream(strs[1])
	if err != nil { return CriLog{}, err }
	time, err := time.Parse(time.RFC3339Nano, strs[0])
	if err != nil { return CriLog{}, err }
	return CriLog {
		Content: strs[3],
		Stream: stream,
		Flags: strs[2],
		Timestamp: time,
	}, nil
}

func parseStream(stream string) (LogStream, error) {
	switch (stream) {
	case "stdout":
		return Stdout, nil
	case "stderr":
		return Stderr, nil
	default:
		// TODO replace this error
		panic("unsupported log type")
	}
}
