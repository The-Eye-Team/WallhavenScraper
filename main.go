package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
	"github.com/labstack/gommon/color"
)

// Arguments struct hold arguments params
var arguments = struct {
	Concurrency int
	Output      string
	CSV         string
}{}

// CSVWriter implement mutex for concurrent CSV editing
type CSVWriter struct {
	mutex     *sync.Mutex
	csvWriter *csv.Writer
}

var checkPre = color.Yellow("[") + color.Green("✓") + color.Yellow("]")
var crossPre = color.Yellow("[") + color.Red("✗") + color.Yellow("]")

var client = http.Client{
	CheckRedirect: func(r *http.Request, via []*http.Request) error {
		r.URL.Opaque = r.URL.Path
		return nil
	},
}

func downloadFile(URL string, file *os.File, client *http.Client) error {
	resp, err := client.Get(URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	defer file.Close()
	if err != nil {
		return err
	}
	return nil
}

func downloadWallpaper(index string, writer *CSVWriter, worker *sync.WaitGroup) {
	defer worker.Done()

	var tags []string
	var uploader, uploadDate, category string

	// Create collector
	c := colly.NewCollector()

	// Randomize user agent on every request
	extensions.RandomUserAgent(c)

	// Scrape tags
	c.OnHTML("ul#tags", func(e *colly.HTMLElement) {
		e.ForEach("li.tag", func(_ int, el *colly.HTMLElement) {
			tags = append(tags, el.ChildText("a.tagname"))
		})
	})

	// Scrape uploader data
	c.OnHTML("dd.showcase-uploader", func(e *colly.HTMLElement) {
		// Scrape username
		uploader = e.ChildText("a.username")
		// Scrape publication date
		uploadDate = e.ChildAttr("time", "datetime")
		// Scrape category
		category = e.ChildText("dt::next")
	})

	// Download the picture
	os.MkdirAll(arguments.Output, os.ModePerm)
	pictureFile, err := os.Create(arguments.Output + "/" + index + ".jpg")
	if err != nil {
		log.Println("Unable to create the file:", err)
	}
	err = downloadFile("https://wallpapers.wallhaven.cc/wallpapers/full/wallhaven-"+index+".jpg", pictureFile, &client)
	if err != nil {
		log.Println("Unable to download the file:", err)
	}

	// Log on request
	c.OnRequest(func(r *colly.Request) {
		fmt.Println(checkPre+color.Green(" Scraping"), r.URL)
	})

	// Log on error
	c.OnError(func(_ *colly.Response, err error) {
		log.Println("Something went wrong:", err)
		runtime.Goexit()
	})

	// Visit page and fill collector
	c.Visit("https://alpha.wallhaven.cc/wallpaper/" + index)

	// Write metadata to CSV
	writer.Write([]string{
		index,
		strings.Join(tags, ","),
		"",
		uploader,
		category,
		"",
		"",
		"",
		uploadDate,
		arguments.Output + "/" + index + ".jpg",
		"https://alpha.wallhaven.cc/wallpaper/" + index,
		"https://wallpapers.wallhaven.cc/wallpapers/full/wallhaven-" + index + ".jpg",
	})
	writer.Flush()
}

func main() {
	var worker sync.WaitGroup
	var count int

	// Parse arguments from command line
	parseArgs(os.Args)

	// Create CSV writer
	writer := newCSVWriter(arguments.CSV)

	// Loop through wallhaven's wallpapers
	for index := 1; ; index++ {
		if _, err := os.Stat(arguments.Output + "/" + strconv.Itoa(index) + ".jpg"); os.IsNotExist(err) {
			worker.Add(1)
			count++
			go downloadWallpaper(strconv.Itoa(index), writer, &worker)
		} else {
			fmt.Println(crossPre + color.Red(" File "+color.Green(index)+color.Red(" already downloaded, skipping.")))
		}
		if count == arguments.Concurrency {
			worker.Wait()
			count = 0
		}
	}
}
