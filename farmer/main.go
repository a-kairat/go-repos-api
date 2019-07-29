package main

import (
	"encoding/json"
	"fmt"

	"time"

	"log"
	"regexp"
	"strings"
	"sync"

	"net/http"
	"net/url"

	database "github.com/a-kairat/go-repos-api/db"
	client "github.com/a-kairat/go-repos-api/gh-client"
	"github.com/a-kairat/go-repos-api/structs"
	"github.com/a-kairat/go-repos-api/utils"
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
	err := database.CreateSchema(db)
	utils.HandleErrPanic(err, "CREATE SCHEMA")

	start <- true

	for {
		select {
		case <-start:
			sendTenRequests(start)
		case <-quit:
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
	if reqErr != nil {
		log.Println(reqErr, "REQ ERR")
		return
	}

	_, respErr := gh.DoJson(req, &body)
	if respErr != nil {
		log.Println(respErr, "RESP ERR")
		return
	}

	body.StoreToRedis()
}

func startDependencySearch(start chan bool) {

	keys, _ := redisClient.HKeys("go-api").Result()

	// keys := []string{"sirupsen/logrus"}
	// fmt.Println(len(keys))
	ch := make(chan string, 100)
	wg := &sync.WaitGroup{}
	goroutines := 1

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		// https://stackoverflow.com/questions/25306073/always-have-x-number-of-goroutines-running-at-any-time
		go func() {
			for {
				key, ok := <-ch
				if !ok {
					wg.Done()
					return
				}
				runBFS(key)
			}
		}()
	}

	for _, key := range keys {
		ch <- key
	}

	close(ch)
	wg.Wait()

	fmt.Printf("REQUESTS MADE: %d\n", gh.RequestsMade())

	time.Sleep(time.Hour * 6)
	start <- true
}

func runBFS(key string) {

	item, _ := getItemFromRedis(key)

	modules := getModules(gh.GetRawFile("/repos/" + key + "/contents/go.mod"))
	item.Modules = modules
	database.Insert(db, item)

	seen := make(map[string]bool)

	for len(modules) > 0 {
		childItem := modules[0]

		// if v, ok := seen[childItem.FullName]; ok {
		// 	fmt.Println(v)
		// }

		if _, ok := seen[childItem.FullName]; !ok {

			childModules := getModules(gh.GetRawFile("/repos/" + childItem.FullName + "/contents/go.mod"))

			childItem.Modules = childModules

			database.Insert(db, *childItem)

			seen[childItem.FullName] = true

			modules = append(modules, childModules...)

		}

		if len(modules) > 0 {
			modules = modules[1:]
		}
	}
	fmt.Println(modules, item)

	fmt.Printf("==============DONE: %s==============\n", key)
}

func getItemFromRedis(key string) (structs.Item, error) {

	var item structs.Item

	object, redisErr := redisClient.HGet("go-api", key).Result()

	if redisErr != nil {
		fmt.Println(redisErr)
		return item, nil
	}

	unmarshalErr := json.Unmarshal([]byte(object), &item)

	if unmarshalErr != nil {
		fmt.Println(key, item)
		log.Println(unmarshalErr, "UNMARSHALL ERR")
		return item, unmarshalErr
	}

	return item, nil
}

func getModules(input string, ghErr error) []*structs.Item {
	if ghErr != nil {
		fmt.Println(input)
		log.Println(ghErr, "GH ERROR")
	}

	result := []*structs.Item{}

	if !strings.HasPrefix(input, `{"message":"Not Found"`) {

		set := make(map[string]bool)
		regex := regexp.MustCompile(`(^.*)?github\.com\/([-_\w]+\/[-_\w]+)`)

		for _, line := range strings.Split(input, "\n") {
			line = strings.TrimSpace(line)

			if strings.HasPrefix(line, "module") {
				continue
			}

			matches := regex.FindStringSubmatch(line)

			if len(matches) > 0 {
				// fmt.Println(matches[2])
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

func createItem(key string) (structs.Item, error) {
	var item structs.Item

	req, reqErr := gh.Request("GET", "/repos/"+key, "", nil)

	if reqErr != nil {
		log.Println(reqErr, "REQ ERR 2")
		return item, reqErr
	}

	resp, respErr := gh.DoJson(req, &item)

	if respErr != nil {
		log.Println(respErr, "RESP ERR 2")
		return item, respErr
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return item, nil
	}

	return item, fmt.Errorf("Error in reponse")
}
