package structs

import (
	"encoding/json"

	"log"
	"strings"

	"github.com/go-redis/redis"
)

var (
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
)

// Owner represents data about repo owner
type Owner struct {
	AvatarURL string `json:"avatar_url"`
}

// Items slice contains all 'necessary' data about reposritories
type Items []struct {
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	HTMLURL         string `json:"html_url"`
	Description     string `json:"description"`
	StargazersCount int    `json:"stargazers_count"`
	ForksCount      int    `json:"forks_count"`
	Owner           Owner  `json:"owner"`
}

// Body struct. Go version of http.response.Body received from
// GitHub.
type Body struct {
	TotalCount        int   `json:"total_count"`
	IncompleteResults bool  `json:"incomplete_results"`
	Items             Items `json:"items"`
}

// Item is a single reposritory item
type Item struct {
	Name            string  `json:"name"`
	FullName        string  `json:"full_name"`
	HTMLURL         string  `json:"html_url"`
	Description     string  `json:"description"`
	StargazersCount int     `json:"stargazers_count"`
	ForksCount      int     `json:"forks_count"`
	Owner           Owner   `json:"owner"`
	Readme          string  `json:"readme"`
	Modules         []*Item `json:"modules"`
	ReadmeIsSet     bool
}

// StoreToRedis stores received repos to redis
func (data *Body) StoreToRedis() error {
	for _, v := range data.Items {

		jsonData, jsonErr := json.Marshal(v)
		if jsonErr != nil {
			log.Fatalln(jsonErr)
		}

		redisClient.HSet("go-api", strings.ToLower(v.FullName), jsonData)
	}

	return nil
}

// Normalize sets Name, FullName and Description fields to lower case.
// Sets Description field to Title case.
func (item *Item) Normalize() {
	item.Name = strings.ToLower(item.Name)
	item.FullName = strings.ToLower(item.FullName)
	item.Description = strings.Title(strings.ToLower(item.Description))
}

func (item *Item) SetReadme(readme string) {
	item.Readme = readme
	item.ReadmeIsSet = true
}
