package main

import (
	"encoding/csv"
	"log"
	"os"
	"sync"
)

func newCSVWriter(fileName string) *CSVWriter {
	if _, err := os.Stat(arguments.CSV); os.IsNotExist(err) {
		csvFile, err := os.Create(arguments.CSV)
		if err != nil {
			log.Fatalf("Cannot create file %q: %s\n", arguments.CSV, err)
			os.Exit(1)
		}
		defer csvFile.Close()
		w := csv.NewWriter(csvFile)
		defer w.Flush()
		// Write CSV header
		w.Write([]string{"ID", "Tags", "NSFW", "Uploader", "Category", "Views", "Size", "Favorites", "UploadDate", "Path", "URL", "ImageURL"})
		return &CSVWriter{csvWriter: w, mutex: &sync.Mutex{}}
	}
	csvFile, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("Cannot open file %q: %s\n", arguments.CSV, err)
		os.Exit(1)
	}
	w := csv.NewWriter(csvFile)
	return &CSVWriter{csvWriter: w, mutex: &sync.Mutex{}}
}

func (w *CSVWriter) Write(row []string) {
	w.mutex.Lock()
	w.csvWriter.Write(row)
	w.mutex.Unlock()
}

func (w *CSVWriter) Flush() {
	w.mutex.Lock()
	w.csvWriter.Flush()
	w.mutex.Unlock()
}
