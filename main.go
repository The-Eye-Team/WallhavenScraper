package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
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
	Cookie      string
	MinID       int
}{}

var checkPre = color.Yellow("[") + color.Green("✓") + color.Yellow("]")
var crossPre = color.Yellow("[") + color.Red("✗") + color.Yellow("]")

var client = http.Client{}

var shouldExit int32
var workers sync.WaitGroup
var jobs chan string
var results chan []string

func init() {
	// Disable HTTP/2: Empty TLSNextProto map
	client.Transport = http.DefaultTransport
	client.Transport.(*http.Transport).TLSNextProto =
		make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
}

// Veri ugly code
func downloadFile(URL string, index string, client *http.Client) (finalURL string, err error) {
	err = tryDownloadFile(URL, index, client)
	if err == nil { return URL, nil }
	// Ugly af
	if err.Error() == "404" && strings.HasSuffix(URL, ".jpg") {
		return downloadFile(strings.TrimSuffix(URL, ".jpg") + ".png", index, client)
	}
	return "", err
}

func tryDownloadFile(URL string, index string, client *http.Client) error {
	// Fetch the data from the URL
	resp, err := client.Get(URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return errors.New("404")
	}

	// Create the file
	file, err := os.Create(arguments.Output + "/" + index + path.Ext(URL))
	if err != nil {
		log.Println("Unable to create the file:", err)
		return err
	}

	// Write the data to the file
	_, err = io.Copy(file, resp.Body)
	defer file.Close()
	if err != nil {
		return err
	}

	return nil
}

func downloadWallpaper(index string) {
	var tags []string
	var uploader, uploadDate, category, size, views, favorites, NSFW, imageURL string

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

	// Scrape picture link
	c.OnHTML("img#wallpaper", func(e *colly.HTMLElement) {
		imageURL = "https:" + e.Attr("src")
	})

	// Log on request
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Cookie", arguments.Cookie)
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
			downloadWallpaper(index)
		default:
			fmt.Println(crossPre+
				color.Yellow(" [")+
				color.Red(index)+
				color.Yellow("]")+
				color.Red(" Something went wrong:"),
				err)
			return
		}
	})

	// Fallback to default jpeg path
	if imageURL == "" {
		imageURL = "https://wallpapers.wallhaven.cc/wallpapers/full/wallhaven-" + index + ".jpg"
	}

	c.SetRequestTimeout(10 * time.Second)

	// Visit page and fill collector
	c.Visit("https://alpha.wallhaven.cc/wallpaper/" + index)

	// Create the file and download the picture
	os.MkdirAll(arguments.Output, os.ModePerm)
	imageURL, err := downloadFile(imageURL, index, &client)
	if err != nil {
		log.Println("Unable to download the file:", err)
		return
	}

	// Write metadata to CSV
	results <- []string{
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
		imageURL,
	}
}

func main() {
	// Create Ctrl+C Handler
	go listenCtrlC()

	// Parse arguments from command line
	parseArgs(os.Args)

	// Create CSV writer channel
	results = make(chan []string)
	defer close(results)
	go writer(results)

	// Start workers
	jobs = make(chan string)
	for i := 0; i < arguments.Concurrency; i++ {
		go worker()
	}

	// Loop through wallhaven's wallpapers
	for index := arguments.MinID; ; index++ {
		// Check if exit requested
		if atomic.LoadInt32(&shouldExit) != 0 {
			break
		}

		if _, err := os.Stat(arguments.Output + "/" + strconv.Itoa(index) + ".jpg"); os.IsNotExist(err) {
			workers.Add(1)
			jobs <- strconv.Itoa(index)
		} else {
			fmt.Println(crossPre + color.Yellow(" [") +
				color.Red(index) +
				color.Yellow("]") +
				color.Red(" File ") +
				color.Green(index) +
				color.Red(" already downloaded, skipping."))
		}
	}

	close(jobs)

	workers.Wait()
}

func worker() {
	for job := range jobs {
		downloadWallpaper(job)
		workers.Done()
	}
}

func listenCtrlC() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	fmt.Fprintln(os.Stderr, "\nExit requested... Waiting for images to DL")
	atomic.StoreInt32(&shouldExit, 1)
	<-c
	fmt.Fprintln(os.Stderr, "\nForce exit")
	os.Exit(255)
}
