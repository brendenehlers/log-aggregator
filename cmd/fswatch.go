package main

import (
	"fmt"
	"os"
	"io/fs"
	"log"
	"time"
)

func main() {
	fmt.Println("hello")
	// dir to watch
	root := "tmp"
	fSystem := os.DirFS(root)
	// file offsets will live outside hot loop, eventually connected to some sort of persistant database
	fileOffsets := make(map[string]int)
	
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
		fileOffsets = reconcileFileOffsets(files, fileOffsets)

		for k, v := range fileOffsets {
			fileOffsets[k] = v + 1
			fmt.Println(k, fileOffsets[k])
		}

		time.Sleep(time.Second)
	}
}

func reconcileFileOffsets(files map[string]interface{}, fileOffsets map[string]int) (map[string]int) {
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

	return fileOffsets
}
