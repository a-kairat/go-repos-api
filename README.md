## [Go repositories](http://167.99.235.172/repos) ##
This is an application that pulls `Go` repositories from GitHub, stores to a database and serves requests to display that data. 
#

### Farmer ###
Farmer is an automated task. Its goal is to fetch repositories and all `Go` modules and `readme` files for each repository.

**Farmer's cycle is the following**:
1. Fetch [up to 1000](https://developer.github.com/v3/search/) Go repositories by making 10 GitHub calls, 100 repositroies per each request (repositories sorted by stars count in descending oredr).
2. Store all that data to Redis. Single data example:
```go
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
	ReadmeIsSet     bool	`json:"readme_is_set"`
}
```
3. Search dependency for each one of the `Item` stored in redis using little bit modified BFS algorithm.
* Get raw `go.mod` in string format, if exists. Filter string by leaving only those that hosted on github. 
* Create `Item` from each dependency by making GitHub calls.
* Store `Item` and its dependencies to DB.
* Put each dependency to a queue. 
* Run the same cycle on the next in queue.
4. When done, sleep for a 6 hours and then start all over again.

### GH client ###
GH client is a package with GitHub requests sending methods. It counts made requests. When requests count is about to reach its limit, it goes to sleep until limit is reset.

### Database ###
Database is a database access package. It creates two tables: `repository` and relation between them `repository to repository`.

```go
type Repo struct {
	ID              int
	Name            string 
	FullName        string 
	HTMLURL         string 
	Description     string 
	StargazersCount int    
	ForksCount      int    
	AvatarURL       string 
	Readme          string 
	Modules         []Repo 
}

type RepoToRepos struct {
	RepoID   int
	ModuleID int
}
```

### HTTP server ###
HTTP server serves http requests and caches "heavy" requests.


### WS server ###
UI component is connected to WS server. Using this connection WS server reads search terms and respond to them.