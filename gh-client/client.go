package client

import (
	"bytes"
	"encoding/json"
	"fmt"

	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/a-sube/go-repos-api/utils"
)

var GH = GitHubClient{
	ghClient: &http.Client{},
	ghURL: &url.URL{
		Scheme: "https",
		Host:   "api.github.com",
	},
}

// GitHubClient is a github http client
type GitHubClient struct {
	ghClient  *http.Client
	ghURL     *url.URL // string // "https://api.github.com/"
	limit     int
	requests  int
	resetTime int64
	initial   bool
}

func (gh *GitHubClient) checkLimit() {
	if gh.limit <= 2 {
		timeLeft := gh.resetTime - time.Now().Unix()
		time.Sleep(time.Second * time.Duration(timeLeft))
	}
}

func (gh *GitHubClient) setLimit(xRemaining, xTimeReset string) {
	limit, _ := utils.StrToInt(xRemaining)
	reset, _ := utils.StrToInt(xTimeReset)
	gh.limit = limit
	gh.resetTime = int64(reset)
}

func (gh *GitHubClient) RequestsMade() int {
	return gh.requests
}

func (gh *GitHubClient) Request(method, path, query string, body interface{}) (*http.Request, error) {

	rel := &url.URL{Path: path}
	url := gh.ghURL.ResolveReference(rel)

	if query != "" {
		url.RawQuery = query
		gh.initial = true
	} else {
		gh.initial = false
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

	req.Header.Set("Authorization", "token "+utils.ACCESSTOKEN)

	return req, nil
}

func (gh *GitHubClient) DoJson(req *http.Request, v interface{}) (*http.Response, error) {

	gh.checkLimit()

	resp, respErr := gh.ghClient.Do(req)
	if respErr != nil {
		return nil, respErr
	}

	defer resp.Body.Close()
	jsonErr := json.NewDecoder(resp.Body).Decode(v)

	// in case we are not doing initial 10 requests
	if !gh.initial {
		if len(resp.Header["X-Ratelimit-Remaining"]) > 0 &&
			len(resp.Header["X-Ratelimit-Reset"]) > 0 {
			gh.setLimit(
				resp.Header["X-Ratelimit-Remaining"][0],
				resp.Header["X-Ratelimit-Reset"][0],
			)
		}
		// gh.LogRequest()
	}
	gh.requests++
	return resp, jsonErr
}

func (gh *GitHubClient) DoRaw(req *http.Request, v interface{}) (string, error) {

	gh.checkLimit()

	resp, respErr := gh.ghClient.Do(req)
	if respErr != nil {
		return "", respErr
	}

	defer resp.Body.Close()

	body, bodyErr := ioutil.ReadAll(resp.Body)
	if bodyErr != nil {
		return "", bodyErr
	}

	if !gh.initial {
		if len(resp.Header["X-Ratelimit-Remaining"]) > 0 &&
			len(resp.Header["X-Ratelimit-Reset"]) > 0 {
			gh.setLimit(
				resp.Header["X-Ratelimit-Remaining"][0],
				resp.Header["X-Ratelimit-Reset"][0],
			)
		}
		// gh.LogRequest()
	}
	gh.requests++
	return string(body), nil
}

func (gh *GitHubClient) GetRawContent(path string) (string, error) {
	req, reqErr := gh.Request("GET", path, "", nil)
	if reqErr != nil {
		return "", reqErr
	}

	req.Header.Set("Accept", "application/vnd.github.v3.raw")
	respString, respErr := gh.DoRaw(req, nil)

	return respString, respErr
}

func (gh *GitHubClient) GetHTML(path string) (string, error) {
	req, reqErr := gh.Request("GET", path, "", nil)
	if reqErr != nil {
		return "", reqErr
	}

	req.Header.Set("Accept", "application/vnd.github.v3.html")
	respString, respErr := gh.DoRaw(req, nil)
	if respErr != nil {
		fmt.Println(path, "FAIL")
	}
	fmt.Println(path, "SUCCESS")
	return respString, respErr
}

func (gh *GitHubClient) LogRequest() {
	timeLeft := gh.resetTime - time.Now().Unix()
	fmt.Printf("requests: %v\tlimit: %v\treset: %v\ttime before reset: %v\n", gh.requests, gh.limit, gh.resetTime, timeLeft)
}
