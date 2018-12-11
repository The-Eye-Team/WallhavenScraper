package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gocolly/colly"
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

var client = http.Client{}

var shouldExit int32 = 0

func init() {
	// Remember cookies
	client.Jar, _ = cookiejar.New(nil)

	// Disable HTTP/2: Empty TLSNextProto map
	client.Transport = http.DefaultTransport
	client.Transport.(*http.Transport).TLSNextProto =
		make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
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
		return
	}

	err = downloadFile("https://wallpapers.wallhaven.cc/wallpapers/full/wallhaven-"+index+".jpg", pictureFile, &client)
	if err != nil {
		log.Println("Unable to download the file:", err)
		return
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
	c.OnError(func(r *colly.Response, err error) {
		switch r.StatusCode {
		case http.StatusUnauthorized:
			fmt.Printf("Not authorized to download %s.\n", index)
		case http.StatusBadGateway:
			time.Sleep(100 * time.Millisecond)
			downloadWallpaper(index, channel, worker)
		default:
			fmt.Println(crossPre+
				color.Yellow(" [")+
				color.Red(index)+
				color.Yellow("]")+
				color.Red(" Something went wrong:"),
				err)
			runtime.Goexit()
		}
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

	// Create Ctrl+C Handler
	go listenCtrlC()

	// Parse arguments from command line
	parseArgs(os.Args)

	// Create CSV writer channel
	channel := make(chan []string)
	defer close(channel)
	go writer(channel)

	// Loop through wallhaven's wallpapers
	for index := 1; ; index++ {
		// Check if exit requested
		if atomic.LoadInt32(&shouldExit) != 0 {
			break
		}

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

func listenCtrlC() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	atomic.StoreInt32(&shouldExit, 1)
}
