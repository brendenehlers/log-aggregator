package main

import (
	"bytes"
	"errors"
	"io"
	"bufio"
	"fmt"
	"os"
	"strings"
	"strconv"
	"time"
	"context"
	"io/fs"
	"log"

	grpclog "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	otellog "go.opentelemetry.io/otel/log"
)

func main() {
	fmt.Println("hello")
	ctx := context.Background()

	root := "var/log/pods"
	//root := "tmp"
	out := make(chan readerResult)

	fSystem := os.DirFS(root)
	// file offsets will live outside hot loop, eventually connected to some sort of persistant database
	// offset is in bytes
	refs := make(map[string]int64)

	go func() {
		provider, err := configureProvider(ctx, "grafana:4317")
		if err != nil { panic(err) }
		defer func() { if err := provider.Shutdown(ctx); err != nil { panic(err) } }()
		logger := provider.Logger("com.behlers.log_scraper")

		// reads from channel and processes new logs
		for {
			select {
			case result := <-out:
				log.Println("got messsage from", result.File)
				// TODO this is really bad, change it be better
				// save current file offsets
				refs[result.File] = result.Offset

				s := bufio.NewScanner(result.Reader)
				for s.Scan() {
					t := s.Text()
					if t == "" { continue }
					criLog, err := parseLog(t)
					// TODO think about recovering from error here
					if err != nil { panic(err) }
					handle(ctx, logger, result.Metadata, criLog)				
				}
			default:
				break
			}
		}
	}()

	for {
		// get the files in the root directory
		files := make(map[string] interface{})
		fs.WalkDir(fSystem, ".", func (path string, d fs.DirEntry, err error) error {
			if err != nil { log.Fatal(err) }
			if d.IsDir() { return nil }
			if f := files[path]; f == nil {
				files[path] = 0
			}
			return nil
		})

		// reconcile files with file offsets
		// TODO cancel goroutines that are deleted
		added, _ := reconcileFileOffsets(files, refs)

		for _, file := range added {
			offset := refs[file]
			go func() {
				metadata, err := parseMetadata(file)
				if err != nil { panic(err) }
				err = fileReader(ctx, root, file, offset, out, metadata)
				if err != nil { 
					// file was deleted, no need to process it anymore
					if errors.Is(err, fs.ErrNotExist) {
						log.Println("handled not found error", err)
						return
					}

					panic(err)
				}
			}()
		}

		time.Sleep(time.Second)
	}
}

type fileRef struct {
	File string
	Offset int64
	CancelFunc context.CancelFunc
}

type readerResult struct {
	Metadata Metadata
	File string
	Offset int64
	Reader io.Reader
}

func fileReader(
	ctx context.Context,
	root, file string,
	offset int64,
	out chan<- readerResult,
	metadata Metadata,
) error {
	oldOffset := offset
	for {
		offset, r, err := readerAt(ctx, root + "/" + file, oldOffset)
		if oldOffset == offset {
			time.Sleep(time.Second)
			continue
		}
		oldOffset = offset
		if err != nil { return err }
		out <- readerResult{
			Metadata: metadata,
			File: file,
			Offset: offset,
			Reader: r,
		}
	}
}

func readerAt(ctx context.Context, file string, off int64) (int64, io.Reader, error) {
	f, err := os.Open(file)
	if err != nil { return 0, nil, err }
	defer f.Close()
	
	offset := off
	var buf bytes.Buffer
	b := make([]byte, 1024)
	for {
		n, err := f.ReadAt(b, offset)
		read := int64(n)
		if err != nil {
			if err != io.EOF {
				return 0, nil, err
			}
			// handle ending data
			if read > 0 {
				// only write populated section of b to buffer
				buf.Write(b[0:read])
				offset = offset + read
			}
			break
		}
		buf.Write(b)
		offset = offset + read
	}

	r := bytes.NewReader(buf.Bytes())
	return offset, r, nil
}

func configureProvider(ctx context.Context, endpoint string) (*sdklog.LoggerProvider, error) {
	// configure otel logging exporter
	exp, err := grpclog.New(ctx, grpclog.WithEndpoint(endpoint), grpclog.WithInsecure())
	if err != nil { return nil, err }
	processor := sdklog.NewBatchProcessor(exp)
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(processor))
	return provider, nil
}

func handle(ctx context.Context, logger otellog.Logger, metadata Metadata, log CriLog) {
	record := otellog.Record{}
	// TODO parse this from the log itself
	var sev otellog.Severity
	var sevText string
	if log.Stream == Stderr {
		sev = otellog.SeverityError1
		sevText = "ERROR"
	} else {
		sev = otellog.SeverityInfo1
		sevText = "INFO"
	}
	record.SetSeverity(sev)
	record.SetSeverityText(sevText)

	record.SetTimestamp(log.Timestamp)
	record.SetObservedTimestamp(time.Now())

	record.SetBody(otellog.StringValue(log.Content))

	record.AddAttributes(
		otellog.String("k8s.namespace.name", metadata.Namespace),
		otellog.String("k8s.pod.name", metadata.PodName),
		otellog.String("k8s.pod.uid", metadata.PodId),
		otellog.String("k8s.container.name", metadata.Container),
		otellog.Int("k8s.container.restart_count", metadata.Restarts),
	)

	logger.Emit(ctx, record)
}

func parseMetadata(file string) (Metadata, error) {
	names := strings.Split(file, "/")
	// default_counter_1544cea7-4641-4a3b-9ccb-702d941295b3
	// TODO validate the structure of the dir
	// TODO are underscores allowed in pod names?
	chunks := strings.Split(names[0], "_")
	restarts, err := strconv.Atoi(strings.Split(names[2], ".")[0])
	if err != nil { return Metadata{}, err }

	return Metadata {
		Namespace: chunks[0],
		PodName: chunks[1],
		PodId: chunks[2],
		Container: names[1],
		Restarts: restarts,
	}, nil
}

func reconcileFileOffsets(files map[string]interface{}, refs map[string]int64) (added, deleted []string) {
	added = make([]string, 0, 10)
	deleted = make([]string, 0, 10)
	// TODO there's gotta be a better way
	// remove deleted files
	// this has to come first, new files are added with an offset of 0
	for k, _ := range refs {
		if f := files[k]; f == nil {
			fmt.Println("removing file offset ", k)
			delete(refs, k)
			deleted = append(deleted, k)
		}
	}
	// add new files
	for k, _ := range files {
		if f := refs[k]; f == 0 {
			fmt.Println("found new file ", k)
			refs[k] = 0
			added = append(added, k)
		}
	}

	return 
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
	Restarts int	
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
