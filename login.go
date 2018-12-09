package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"net/url"
)

func login(user, pass string) error {
	token, err := getLoginCsrf()
	if err != nil { return err }

	res, err := client.PostForm("https://alpha.wallhaven.cc/auth/login", url.Values{
		"_token": {token},
		"username": {user},
		"password": {pass},
	})
	res.Body.Close()

	if res.StatusCode == http.StatusOK {
		fmt.Println(checkPre + " Successfully logged in.")

		return nil
	} else {
		return fmt.Errorf("HTTP %s", res.Status)
	}
}

func getLoginCsrf() (string, error) {
	res, err := client.Get("https://alpha.wallhaven.cc/")
	if err != nil { return "", err }
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil { return "", err }

	s, ok := doc.Find(`input[name="_token"]`).Attr("value")
	if !ok {
		return "", fmt.Errorf("no token found")
	}

	return s, nil
}
