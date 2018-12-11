package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func login(user, pass string) error {
	token, err := getCsrfToken("https://alpha.wallhaven.cc/")
	if err != nil {
		return err
	}

	res, err := client.PostForm("https://alpha.wallhaven.cc/auth/login", url.Values{
		"_token":   {token},
		"username": {user},
		"password": {pass},
	})
	res.Body.Close()

	if res.StatusCode == http.StatusOK {
		fmt.Println(checkPre + " Successfully logged in.")

		return nil
	}
	return fmt.Errorf("HTTP %s", res.Status)
}

func setProfileNsfw() error {
	token, err := getCsrfToken("https://alpha.wallhaven.cc/settings/browsing")
	if err != nil {
		return err
	}

	req, _ := http.NewRequest("PUT", "https://alpha.wallhaven.cc/settings/browsing", strings.NewReader(url.Values{
		"_token":  {token},
		"sfw":     {"sfw"},
		"sketchy": {"sketchy"},
		"nsfw":    {"nsfw"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	res.Body.Close()

	if res.StatusCode == http.StatusOK {
		fmt.Println(checkPre + " Set profile to accept NSFW.")

		return nil
	}
	return fmt.Errorf("HTTP %s", res.Status)
}

func getCsrfToken(url string) (string, error) {
	res, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", err
	}

	s, ok := doc.Find(`input[name="_token"]`).Attr("value")
	if !ok {
		return "", fmt.Errorf("no token found")
	}

	return s, nil
}
