package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/dghubble/oauth1"
)

// All are required.
var TWITTER_CONSUMER_KEY = getEnv("TWITTER_CONSUMER_KEY")
var TWITTER_CONSUMER_SECRET = getEnv("TWITTER_CONSUMER_SECRET")
var TWITTER_ACCESS_TOKEN = getEnv("TWITTER_ACCESS_TOKEN")
var TWITTER_ACCESS_TOKEN_SECRET = getEnv("TWITTER_ACCESS_TOKEN_SECRET")
var UNSPLASH_ACCESS_KEY = getEnv("UNSPLASH_ACCESS_KEY")

const interval = 60 * 1000 // 60 seconds
var oauthConfig = oauth1.NewConfig(TWITTER_CONSUMER_KEY, TWITTER_CONSUMER_SECRET)
var oauthToken = oauth1.NewToken(TWITTER_ACCESS_TOKEN, TWITTER_ACCESS_TOKEN_SECRET)

func getEnv(key string) string {
	result := os.Getenv(key)
	if len(result) == 0 {
		log.Fatalf("%s is required.", key)
	}
	return result
}

type sunInfo struct {
	Rise time.Time
	Set  time.Time
}

func sunApi() sunInfo {
	type sunResponse struct {
		Results struct {
			Sunrise string
			Sunset  string
		}
	}

	bangkokTimeLocation, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		log.Println(err)
	}
	currentTime := time.Now().In(bangkokTimeLocation)
	currentDate := currentTime.Format("2006-01-02")

	resp, err := http.Get(fmt.Sprintf("https://api.sunrise-sunset.org/json?lat=13.7563&lng=100.5018&date=%s&formatted=0", currentDate))
	if err != nil {
		log.Println("sunApi get", err)
		return sunInfo{
			Rise: time.UnixMilli(0),
			Set:  time.UnixMilli(0),
		}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("sunApi read", err)
		return sunInfo{
			Rise: time.UnixMilli(0),
			Set:  time.UnixMilli(0),
		}
	}
	var parsed sunResponse
	json.Unmarshal(body, &parsed)

	rise, err := time.Parse(time.RFC3339, parsed.Results.Sunrise)
	if err != nil {
		log.Println("sunApi parse sunrise", err)
		return sunInfo{
			Rise: time.UnixMilli(0),
			Set:  time.UnixMilli(0),
		}
	}
	set, err := time.Parse(time.RFC3339, parsed.Results.Sunset)
	if err != nil {
		log.Println("sunApi parse sunset", err)
		return sunInfo{
			Rise: time.UnixMilli(0),
			Set:  time.UnixMilli(0),
		}
	}
	return sunInfo{
		Rise: rise,
		Set:  set,
	}
}

func setCover(base64Img string) (string, error) {
	path := "https://api.twitter.com/1.1/account/update_profile_banner.json"
	httpClient := oauthConfig.Client(oauth1.NoContext, oauthToken)
	resp, err := httpClient.PostForm(path, url.Values{
		"banner": {base64Img},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	bodyString := string(body)
	if err != nil {
		return "", err
	} else if resp.StatusCode > 202 || resp.StatusCode < 200 {
		return "", fmt.Errorf("%d:%s", resp.StatusCode, bodyString)
	}
	return fmt.Sprintf("%d:%s", resp.StatusCode, bodyString), nil
}

func unsplashApi() (string, error) {
	type unsplashResponse struct {
		Urls struct{ Full string }
	}
	unsplashClient := http.Client{}
	req, err := http.NewRequest(http.MethodGet, "https://api.unsplash.com/photos/random?topics=6sMVjTLSkeQ", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept-Version", "v1")
	req.Header.Set("Authorization", fmt.Sprintf("Client-ID %s", UNSPLASH_ACCESS_KEY))
	resp, err := unsplashClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var parsed unsplashResponse
	json.Unmarshal(body, &parsed)
	return fmt.Sprintf("%s&w=2000", parsed.Urls.Full), nil
}

func nasaApi() (string, error) {
	type nasaImageResponse struct {
		Media_type string
		Hdurl      string
		Url        string
	}
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("https://api.nasa.gov/planetary/apod?api_key=DEMO_KEY&count=1")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Println("NASA API timeout, retrying with Unsplash...")
			return unsplashApi()
		}
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var parsed []nasaImageResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Println("NASA API Unmarshal error:", err)
		return nasaApi()
	}

	if len(parsed) != 1 {
		return nasaApi()
	}

	image := parsed[0]

	if image.Media_type != "image" {
		return nasaApi()
	}

	if len(image.Hdurl) != 0 {
		return image.Hdurl, nil
	} else {
		return image.Url, nil
	}
}

func downloadImage(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return ""
	}
	return base64.StdEncoding.EncodeToString(body)
}

func triggered(isSunrise bool) {
	var url string
	var err error
	if isSunrise {
		log.Println("Triggered: sunrise")
		url, err = unsplashApi()
	} else {
		log.Println("Triggered: sunset")
		url, err = nasaApi()
	}
	if err != nil {
		log.Println(err)
		log.Println("Will retry...")
		time.Sleep(interval * time.Millisecond)
		triggered(isSunrise)
		return
	}
	var resp string
	resp, err = setCover(downloadImage(url))
	if err != nil {
		log.Println(err)
		log.Println("Will retry...")
		time.Sleep(interval * time.Millisecond)
		triggered(isSunrise)
		return
	} else {
		log.Println(resp)
	}
}

func run() {
	sun := sunApi()
	now := time.Now()
	isSunrise := math.Abs(float64(sun.Rise.Sub(now).Milliseconds())) < interval/2
	isSunset := math.Abs(float64(sun.Set.Sub(now).Milliseconds())) < interval/2

	if isSunrise || isSunset {
		triggered(isSunrise)
	}
}

func main() {
	run()
	for range time.Tick(interval * time.Millisecond) {
		run()
	}
}
