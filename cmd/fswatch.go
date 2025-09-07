package main

import (
	"fmt"
	"os"
	"io/fs"
	"log"
	"time"
	"bytes"
	"io"
	"bufio"
)

func main() {
	fmt.Println("hello")
	// dir to watch
	root := "tmp"
	fSystem := os.DirFS(root)
	// file offsets will live outside hot loop, eventually connected to some sort of persistant database
	// offset is in bytes
	fileOffsets := make(map[string]int64)
	
	for {
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
		reconcileFileOffsets(files, fileOffsets)

		for file, offset := range fileOffsets {
			newOffset, lines, err := readFileAtOffset(root + "/" + file, offset)
			if err != nil { panic(err) }
			fileOffsets[file] = newOffset
			fmt.Println(lines)
		}

		time.Sleep(time.Second)
	}
}

func readLinesAt(file string, off int64) (int64, []string, error) {
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

	s := bufio.NewScanner(bytes.NewReader(buf.Bytes()))

	lines := make([]string, 0, 10)
	for s.Scan() {
		line := s.Text()
		lines = append(lines, line)
	}

	return offset, lines, nil
}

func reconcileFileOffsets(files map[string]interface{}, fileOffsets map[string]int64) {
	// TODO there's gotta be a better way
	// remove deleted files
	// this has to come first, new files are added with an offset of 0
	for k, _ := range fileOffsets {
		if f := files[k]; f == nil {
			fmt.Println("removing file offset ", k)
			delete(fileOffsets, k)
		}
	}
	// add new files
	for k, _ := range files {
		if f := fileOffsets[k]; f == 0 {
			fmt.Println("found new file ", k)
			fileOffsets[k] = 0
		}
	}
}
