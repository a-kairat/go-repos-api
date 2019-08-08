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

	database "github.com/a-sube/go-repos-api/db"
	client "github.com/a-sube/go-repos-api/gh-client"

	"github.com/a-sube/go-repos-api/structs"
	"github.com/a-sube/go-repos-api/utils"
	"github.com/go-redis/redis"
)

var (
	gh = client.GH

	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	queryParameter = "q=go+package+in:readme+language:go&sort=stars&order=desc&page="
)

func main() {

	utils.CheckEnvVars(true, true, true)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	go func() {
		s := <-sigs
		log.Printf("RECEIVED SIGNAL: %s", s)
		os.Exit(1)
	}()

	database.CreateSchema()

	sendTenRequests()
}

func sendTenRequests() {

	wg := &sync.WaitGroup{}
	for i := 1; i <= 10; i++ {
		wg.Add(1)
		go sendSingleRequest(i, wg)
	}
	wg.Wait()

	startDependencySearch()

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

func startDependencySearch() {

	keys, _ := redisClient.HKeys("go-api").Result()

	for _, key := range keys {
		runBFSlike(key)
	}

	log.Printf("CYCLE DONE! REQUESTS MADE: %d\n", gh.RequestsMade())

	keys = []string{}

	time.Sleep(time.Hour * 6)
	sendTenRequests()
}

func runBFSlike(key string) {

	item, _ := getItemFromRedis(key)
	rawFiles, err := gh.GetRawContent("/repos/" + key + "/contents/go.mod")
	utils.HandleErrPANIC(err, "GetRawContent")

	modules := getModules(rawFiles, key)
	item.Modules = modules

	readmeFile := getReadmeHTML(key)
	item.Readme = readmeFile

	item.Normalize()

	database.Insert(item)

	seen := make(map[string]bool)

	for len(modules) > 0 {
		childItem := modules[0]

		if _, ok := seen[childItem.FullName]; !ok {
			childRawFiles, err := gh.GetRawContent("/repos/" + childItem.FullName + "/contents/go.mod")
			utils.HandleErrPANIC(err, "GetRawContent")

			childModules := getModules(childRawFiles, childItem.FullName)
			childItem.Modules = childModules

			if childItem.Readme == "" {
				childReadmeFile := getReadmeHTML(childItem.FullName)
				childItem.Readme = childReadmeFile
			}

			childItem.Normalize()

			database.Insert(*childItem)

			seen[childItem.FullName] = true

			modules = append(modules, childModules...)
		}

		modules = modules[1:]

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

// getModules takes string input (example: https://github.com/hashicorp/consul/blob/master/go.mod). Returns slice of
// key - owner/repo format. (example: hashicorp/consul)
func getModules(input string, key string) []*structs.Item {

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
				item.Readme = getReadmeHTML(key)
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

func getReadmeHTML(key string) string {
	readme, err := gh.GetHTML("/repos/" + key + "/readme")
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return readme
}
