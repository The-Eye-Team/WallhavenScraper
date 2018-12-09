package main

import (
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

var checkPre = color.Yellow("[") + color.Green("✓") + color.Yellow("]")
var crossPre = color.Yellow("[") + color.Red("✗") + color.Yellow("]")

var client = http.Client{
	CheckRedirect: func(r *http.Request, via []*http.Request) error {
		r.URL.Opaque = r.URL.Path
		return nil
	},
}

func downloadFile(URL string, file *os.File, client *http.Client) error {
	// Fetch the data from the URL
	resp, err := client.Get(URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the data to the file
	_, err = io.Copy(file, resp.Body)
	defer file.Close()
	if err != nil {
		return err
	}
	return nil
}

func downloadWallpaper(index string, channel chan<- []string, worker *sync.WaitGroup) {
	defer worker.Done()

	var tags []string
	var uploader, uploadDate, category, size, views, favorites, NSFW string

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

	// Scrape NSFW tag
	c.OnHTML("form#wallpaper-purity-form", func(e *colly.HTMLElement) {
		NSFW = e.ChildText("label.purity")
	})

	// Scrape uploader data
	c.OnHTML("dd.showcase-uploader", func(e *colly.HTMLElement) {
		// Scrape username
		uploader = e.ChildText("a.username")
		// Scrape publication date
		uploadDate = e.ChildAttr("time", "datetime")
	})
	c.OnHTML("div.sidebar-section", func(e *colly.HTMLElement) {
		e.ForEach("dd", func(_ int, el *colly.HTMLElement) {
			// Scrape category
			prev := el.DOM.Prev()
			if prev.Text() == "Category" {
				category = el.DOM.Text()
			}

			// Scrape size
			if prev.Text() == "Size" {
				size = el.DOM.Text()
			}

			// Scrape views
			if prev.Text() == "Views" {
				views = el.DOM.Text()
			}

			// Scrape favorites
			if prev.Text() == "Favorites" {
				favorites = el.DOM.Text()
			}
		})
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
		fmt.Println(checkPre+
			color.Yellow(" [")+
			color.Green(index)+
			color.Yellow("]")+
			color.Green(" Scraping"),
			r.URL)
	})

	// Log on error
	c.OnError(func(_ *colly.Response, err error) {
		fmt.Println(crossPre+
			color.Yellow(" [")+
			color.Red(index)+
			color.Yellow("]")+
			color.Red(" Something went wrong:"),
			err)
		runtime.Goexit()
	})

	// Visit page and fill collector
	c.Visit("https://alpha.wallhaven.cc/wallpaper/" + index)

	// Write metadata to CSV
	channel <- []string{
		index,
		strings.Join(tags, ","),
		NSFW,
		uploader,
		category,
		views,
		size,
		favorites,
		uploadDate,
		arguments.Output + "/" + index + ".jpg",
		"https://alpha.wallhaven.cc/wallpaper/" + index,
		"https://wallpapers.wallhaven.cc/wallpapers/full/wallhaven-" + index + ".jpg",
	}
}

func main() {
	var worker sync.WaitGroup
	var count int

	// Parse arguments from command line
	parseArgs(os.Args)

	// Create CSV writer channel
	channel := make(chan []string)
	defer close(channel)
	go writer(channel)

	// Loop through wallhaven's wallpapers
	for index := 1; ; index++ {
		if _, err := os.Stat(arguments.Output + "/" + strconv.Itoa(index) + ".jpg"); os.IsNotExist(err) {
			worker.Add(1)
			count++
			go downloadWallpaper(strconv.Itoa(index), channel, &worker)
		} else {
			fmt.Println(crossPre + color.Yellow(" [") +
				color.Red(index) +
				color.Yellow("]") +
				color.Red(" File ") +
				color.Green(index) +
				color.Red(" already downloaded, skipping."))
		}
		if count == arguments.Concurrency {
			worker.Wait()
			count = 0
		}
	}
}