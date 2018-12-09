package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/akamensky/argparse"
)

func parseArgs(args []string) {
	// Create new parser object
	parser := argparse.NewParser("WallhavenScraper", "Scraper for wallhaven.cc")

	// Create flags
	concurrency := parser.Int("j", "concurrency", &argparse.Options{
		Required: false,
		Help:     "Number of concurrent connection",
		Default:  4})

	output := parser.String("o", "output", &argparse.Options{
		Required: false,
		Help:     "Output folder for images",
		Default:  "./Downloads"})

	csv := parser.String("", "csv", &argparse.Options{
		Required: false,
		Help:     "CSV for writing metadata",
		Default:  "wallhaven.csv"})

	// Parse input
	err := parser.Parse(args)
	if err != nil {
		// In case of error print error and print usage
		// This can also be done by passing -h or --help flags
		fmt.Print(parser.Usage(err))
		os.Exit(0)
	}

	// Convert path parameters to absolute paths
	if *output != "" {
		arguments.Output, _ = filepath.Abs(*output)
	}

	arguments.Concurrency = *concurrency
	arguments.CSV = *csv
}
