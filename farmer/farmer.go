package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"time"

	"log"
	"regexp"
	"strings"
	"sync"

	"net/http"
	"net/url"

	database "github.com/a-sube/go-repos-api/db"
	client "github.com/a-sube/go-repos-api/gh-client"

	"github.com/a-sube/go-repos-api/structs"
	"github.com/a-sube/go-repos-api/utils"
	"github.com/go-redis/redis"
)

var (
	gh = client.GitHubClient{
		GHclient: &http.Client{},
		GHURL: &url.URL{
			Scheme: "https",
			Host:   "api.github.com",
		},
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	db = database.Connect()

	queryParameter = "q=go+package+in:readme+language:go&sort=stars&order=desc&page="

	start = make(chan bool, 1)
	quit  = make(chan bool, 1)
)

func main() {

	utils.CheckEnvVars()

	err := database.CreateSchema(db)
	utils.HandleErrEXIT(err, "CREATE SCHEMA")

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs)

	go func() {
		s := <-sigs
		log.Printf("RECEIVED SIGNAL: %s", s)
		quitFunc()
		os.Exit(1)
	}()

	start <- true

	for {
		select {
		case <-start:
			sendTenRequests(start)
		case <-quit:
			close(start)
			close(start)
			return
		}
	}
}

func quitFunc() {
	quit <- true
}

func sendTenRequests(start chan bool) {

	wg := &sync.WaitGroup{}
	for i := 1; i <= 10; i++ {
		wg.Add(1)
		go sendSingleRequest(i, wg)
	}
	wg.Wait()

	startDependencySearch(start)
}

func sendSingleRequest(page int, wg *sync.WaitGroup) {

	defer wg.Done()

	var body structs.Body

	req, reqErr := gh.Request(
		"GET",
		"/search/repositories",
		queryParameter+utils.IntToStr(page)+"&per_page=100",
		nil,
	)
	utils.HandleErrEXIT(reqErr, "REQ ERR")

	_, respErr := gh.DoJson(req, &body)
	utils.HandleErrEXIT(respErr, "RESP ERR")

	body.StoreToRedis()
}

func startDependencySearch(start chan bool) {

	keys, _ := redisClient.HKeys("go-api").Result()

	for _, key := range keys {
		runBFSlike(key)
	}

	log.Printf("CYCLE DONE! REQUESTS MADE: %d\n", gh.RequestsMade())

	keys = []string{}

	time.Sleep(time.Hour * 6)
	start <- true
}

func runBFSlike(key string) {

	item, _ := getItemFromRedis(key)
	rawFiles, err := gh.GetRawFile("/repos/" + key + "/contents/go.mod")
	modules := getModules(rawFiles, err, key)
	item.Modules = modules
	item.Normalize()

	database.Insert(db, item)

	seen := make(map[string]bool)

	for len(modules) > 0 {
		childItem := modules[0]

		if _, ok := seen[childItem.FullName]; !ok {
			childRawFiles, err := gh.GetRawFile("/repos/" + childItem.FullName + "/contents/go.mod")

			childModules := getModules(childRawFiles, err, childItem.FullName)

			childItem.Modules = childModules
			childItem.Normalize()
			database.Insert(db, *childItem)

			seen[childItem.FullName] = true

			modules = append(modules, childModules...)

		}

		if len(modules) > 0 {
			modules = modules[1:]
		}
	}
}

func getItemFromRedis(key string) (structs.Item, error) {

	var item structs.Item

	object, redisErr := redisClient.HGet("go-api", key).Result()

	if redisErr != nil {
		utils.HandleErrPANIC(redisErr, "REDIS ERR")
		return item, nil
	}

	unmarshalErr := json.Unmarshal([]byte(object), &item)

	if unmarshalErr != nil {
		utils.HandleErrPANIC(unmarshalErr, "UNMARSHALL ERR")
		return item, unmarshalErr
	}

	return item, nil
}

// input raw file example: https://github.com/hashicorp/consul/blob/master/go.mod
// key - owner/repo. example: hashicorp/consul
func getModules(input string, ghErr error, key string) []*structs.Item {
	if ghErr != nil {
		utils.HandleErrPANIC(ghErr, "GH ERROR")
	}

	result := []*structs.Item{}

	if !strings.HasPrefix(input, `{"message":"Not Found"`) {

		set := make(map[string]bool)
		// grab only modules that start with pattern `github.com/<owner>/<repo>`
		regex := regexp.MustCompile(`(^.*)?github\.com\/([-_\w]+\/[-_\w]+)`)

		for _, line := range strings.Split(input, "\n") {
			line = strings.TrimSpace(line)

			if strings.HasPrefix(line, "module") {
				continue
			}

			matches := regex.FindStringSubmatch(line)

			// if found matches and matched string is not a repo itself
			// add to set
			if len(matches) > 0 && matches[2] != key {
				set[matches[2]] = true
			}
		}

		for key := range set {
			item, itemErr := createItem(strings.ToLower(key))
			if key == "" || itemErr != nil {
				continue
			} else {
				result = append(result, &item)
			}
		}
	}

	return result
}

// Gets repo from github. Returns Item struct or an error
func createItem(key string) (structs.Item, error) {
	var item structs.Item

	req, reqErr := gh.Request("GET", "/repos/"+key, "", nil)

	if reqErr != nil {
		utils.HandleErrPANIC(reqErr, "REQ ERR 2")
		return item, reqErr
	}

	resp, respErr := gh.DoJson(req, &item)

	if respErr != nil {
		utils.HandleErrPANIC(respErr, "RESP ERR 2")
		return item, respErr
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return item, nil
	}

	return item, fmt.Errorf("Error in reponse")
}
