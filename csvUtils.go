package main

import (
	"encoding/csv"
	"log"
	"os"
	"sort"
	"time"

	"github.com/spf13/cast"
)

func openFile(fileName string) (f *os.File, created bool, err error) {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		// CSV not exists, create
		csvFile, err := os.Create(fileName)
		if err != nil {
			log.Fatalf("Cannot create file %q: %s\n", fileName, err)
			os.Exit(1)
		}
		return csvFile, true, nil
	} else if err == nil {
		// CSV exists, append
		csvFile, err := os.Open(fileName)
		if err != nil {
			log.Fatalf("Cannot open file %q: %s\n", fileName, err)
			os.Exit(1)
		}

		return csvFile, false, nil
	} else {
		return nil, false, err
	}
}

func writer(c <-chan []string) {
	csvFile, created, err := openFile(arguments.CSV)
	if err != nil {
		log.Fatalf("Cannot open file %q: %s\n", arguments.CSV, err)
		os.Exit(1)
	}
	defer csvFile.Close()

	w := csv.NewWriter(csvFile)
	w.Flush()

	if created {
		w.Write([]string{"ID", "Tags", "NSFW", "Uploader", "Category", "Views", "Size", "Favorites", "UploadDate", "Path", "URL", "ImageURL"})
	}

	var buf [][]string
	ticker := time.NewTicker(10 * time.Second).C

	for {
		flush := false
		force := false

		select {
		case <-ticker:
			// Executes after 2s pass
			flush = true
			force = true

		case element, ok := <-c:
			// Executes on new object
			if !ok {
				// Channel closed
				goto exitLoop
			}

			// Buffer element
			buf = append(buf, element)

			// Flush buf on buffer overflow
			if len(buf) > arguments.Concurrency*2 {
				flush = true
				force = false
			}
		}

		if flush {
			flushBuf(w, &buf, force)
		}
	}
exitLoop:
	flushBuf(w, &buf, true)
}

func flushBuf(w *csv.Writer, bufPtr *[][]string, force bool) {
	buf := *bufPtr

	sort.Slice(buf, func(i, j int) bool {
		return cast.ToInt(buf[i][0]) < cast.ToInt(buf[j][0])
	})

	if !force {
		firstHalf := buf[:len(buf)/2]
		secondHalf := buf[len(buf)/2:]

		*bufPtr = secondHalf

		for _, e := range firstHalf {
			w.Write(e)
		}
		w.Flush()
	} else {
		for _, e := range buf {
			w.Write(e)
		}
		w.Flush()

		*bufPtr = nil
	}
}
