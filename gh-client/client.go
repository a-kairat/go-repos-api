package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-api/utils"
	"runtime"

	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sync/atomic"
	"time"
)

var (
	accessToken, accessTokenOk = os.LookupEnv("GITHUB_ACCESS_TOKEN")
	search                     int32
	requests                   int32
)

// GitHubClient is a github http client
type GitHubClient struct {
	GHclient *http.Client
	GHURL    *url.URL // string // "https://api.github.com/"
}

func (gh *GitHubClient) Request(method, path, query string, body interface{}) (*http.Request, error) {
	rel := &url.URL{Path: path}
	url := gh.GHURL.ResolveReference(rel)

	if query != "" {
		url.RawQuery = query
		atomic.StoreInt32(&search, 1)
	} else {
		atomic.StoreInt32(&search, 0)
	}

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)

		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, url.String(), buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "token "+accessToken)

	return req, nil
}

func (gh *GitHubClient) DoJson(req *http.Request, v interface{}) (*http.Response, error) {

	resp, respErr := gh.GHclient.Do(req)
	if respErr != nil {
		return nil, respErr
	}

	defer resp.Body.Close()

	if search == 0 {
		if len(resp.Header["X-Ratelimit-Remaining"]) > 0 &&
			len(resp.Header["X-Ratelimit-Reset"]) > 0 {
			checkRateLimit(
				resp.Header["X-Ratelimit-Remaining"][0],
				resp.Header["X-Ratelimit-Reset"][0],
			)
		}
	}

	jsonErr := json.NewDecoder(resp.Body).Decode(v)
	return resp, jsonErr
}

func (gh *GitHubClient) DoRaw(req *http.Request, v interface{}) (string, error) {

	resp, respErr := gh.GHclient.Do(req)
	if respErr != nil {
		return "", respErr
	}

	defer resp.Body.Close()

	if search == 0 {
		if len(resp.Header["X-Ratelimit-Remaining"]) > 0 &&
			len(resp.Header["X-Ratelimit-Reset"]) > 0 {
			checkRateLimit(
				resp.Header["X-Ratelimit-Remaining"][0],
				resp.Header["X-Ratelimit-Reset"][0],
			)
		}
	}

	body, bodyErr := ioutil.ReadAll(resp.Body)
	if bodyErr != nil {
		return "", bodyErr
	}

	return string(body), nil
}

func (gh *GitHubClient) GetRawFile(path string) (string, error) {
	req, reqErr := gh.Request("GET", path, "", nil)
	if reqErr != nil {
		return "", reqErr
	}

	req.Header.Set("Accept", "application/vnd.github.v3.raw")
	respString, respErr := gh.DoRaw(req, nil)

	return respString, respErr
}

func (gh *GitHubClient) RequestsMade() int32 {
	return requests
}

func checkRateLimit(rateLimt, rateLimtReset string) {
	atomic.AddInt32(&requests, 1)
	limit, _ := utils.StrToInt(rateLimt)
	reset, _ := utils.StrToInt(rateLimtReset)
	timeLeft := int64(reset) - time.Now().Unix()

	fmt.Printf("REQUEST: %d\tLIMIT: %d\t RESET: %d\t TIME BEFORE RESET: %d\tRUNNING GOROUTINES: %d\n", requests, limit, reset, timeLeft, runtime.NumGoroutine())

	time.Sleep(setInterval(timeLeft, limit))
}

func setInterval(timeLeft int64, limit int) time.Duration {
	if limit <= 3 {
		return time.Second * time.Duration(timeLeft)
	}

	return time.Second * 0
}

// func main() {
// 	if !accessTokenOk {
// 		log.Fatalln("Can't read access token")
// 	}

// 	gh := GitHubClient{&http.Client{}, &url.URL{
// 		Scheme: "https",
// 		Host:   "api.github.com"}}

// 	gh.getRawFile("/repos/kubernetes/kubernetes/contents/go.mod")
// }
