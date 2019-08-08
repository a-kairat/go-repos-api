package database

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/a-sube/go-repos-api/structs"
	"github.com/a-sube/go-repos-api/utils"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"github.com/go-redis/redis"
)

var (
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	// DB is database
	DB = pg.Connect(&pg.Options{
		User:     utils.DBUSER,
		Password: utils.DBPSWD,
	})
)

// Repo is a table and json response struct
type Repo struct {
	ID              int
	Name            string `json:"name" sql:",nullable"`
	FullName        string `json:"full_name" sql:",unique"`
	HTMLURL         string `json:"html_url" sql:",nullable"`
	Description     string `json:"description" sql:",nullable"`
	StargazersCount int    `json:"stargazers_count" sql:",nullable"`
	ForksCount      int    `json:"forks_count" sql:",nullable"`
	AvatarURL       string `json:"avatar_url" sql:",nullable"`
	Readme          string `json:"readme" sql:",nullable"`
	Modules         []Repo `json:"modules" pg:"many2many:repo_to_repos,joinFK:module_id,zeroable"`
}

// RepoToRepos is a many2many table struct
type RepoToRepos struct {
	RepoID   int
	ModuleID int
}

// DBResponse is a json response struct
type DBResponse struct {
	Count int
	Items []Repo
}

func init() {
	// Register many to many model so ORM can better recognize m2m relation.
	// This should be done before dependant models are used.
	orm.RegisterTable((*RepoToRepos)(nil))
}

// Connect connects to database and returns pointer to it
// func Connect() *pg.DB {

// 	DB := pg.Connect(&pg.Options{
// 		User:     utils.DBUSER,
// 		Password: utils.DBPSWD,
// 	})

// 	return DB
// }

// CreateSchema creates Repo and RepoToRepo tables if not exists.
func CreateSchema() error {
	models := []interface{}{
		(*Repo)(nil),
		(*RepoToRepos)(nil),
	}
	for _, model := range models {
		err := DB.CreateTable(model, &orm.CreateTableOptions{
			// Temp: true,
			IfNotExists: true,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Insert takes `Item` struct, inserts it to Repo table,
// iterates over child modules and inserts each module it to RepoToRepos table.
func Insert(v structs.Item) {
	repo := &Repo{
		Name:            v.Name,
		FullName:        v.FullName,
		HTMLURL:         v.HTMLURL,
		Description:     v.Description,
		StargazersCount: v.StargazersCount,
		ForksCount:      v.ForksCount,
		AvatarURL:       v.Owner.AvatarURL,
		Readme:          v.Readme,
	}

	_, err := DB.Model(repo).
		OnConflict("(full_name) DO UPDATE").
		Insert()

	utils.HandleErrEXIT(err, "DB REPO INSERT")

	for _, mod := range v.Modules {

		module := &Repo{
			Name:            mod.Name,
			FullName:        mod.FullName,
			HTMLURL:         mod.HTMLURL,
			Description:     mod.Description,
			StargazersCount: mod.StargazersCount,
			ForksCount:      mod.ForksCount,
			AvatarURL:       mod.Owner.AvatarURL,
			Readme:          mod.Readme,
		}

		_, err := DB.Model(module).
			OnConflict("(full_name) DO UPDATE").
			Insert()

		utils.HandleErrEXIT(err, "DB MODULE INSERT")

		repoToModule := &RepoToRepos{RepoID: repo.ID, ModuleID: module.ID}

		_, err = DB.Model(repoToModule).
			Where("repo_id = ?repo_id").
			Where("module_id = ?module_id").
			SelectOrInsert()

		utils.HandleErrEXIT(err, "DB REPO TO REPOS SELECT OR INSERT")

	}

}

// SelectLimitOffset is a paginator. Selects limited items per page.
// Default limit is 10.
func SelectLimitOffset(page, limit string) ([]Repo, error) {
	if limit == "" {
		limit = "10"
	}

	p, _ := utils.StrToInt(page)
	l, _ := utils.StrToInt(limit)
	offset := l * (p - 1)

	var repos []Repo

	err := DB.Model(&repos).
		Column("id", "name", "description", "stargazers_count", "forks_count", "avatar_url").
		Order("stargazers_count DESC NULLS LAST").
		Limit(l).
		Offset(offset).
		Select()

	utils.HandleErrEXIT(err, "SELECT WITH LIMIT AND OFFSET")

	return repos, nil
}

// SelectALLByName selects all reposritories from table that have name = name.
func SelectALLByName(name string) string {

	result := []Repo{}

	err := DB.Model(&result).
		Column("id", "name", "full_name", "htmlurl", "stargazers_count", "forks_count", "description").
		Where("name = ?", name).
		Order("stargazers_count DESC NULLS LAST").
		Select()

	utils.HandleErrEXIT(err, "SELECT BY NAME")

	dbResponse := DBResponse{
		Count: len(result),
		Items: result,
	}

	j, _ := json.MarshalIndent(dbResponse, "", "  ")

	return string(j)
}

// SelectByID selects all reposritories from table that have id = id.
func SelectByID(id string) string {

	var result Repo

	err := DB.Model(&result).
		Column("id", "name", "full_name", "htmlurl", "stargazers_count", "forks_count", "description", "avatar_url").
		Where("id = ?", id).
		Order("stargazers_count DESC NULLS LAST").
		Select()

	if err != nil {
		return ""
	}

	j, _ := json.MarshalIndent(result, "", "  ")

	return string(j)

}

// SelectByIDWithModules selects single module and its child modules.
func SelectByIDWithModules(id, l string) string {

	level, levelErr := utils.StrToInt(l)

	if levelErr != nil {
		return ""
	}

	var result Repo

	err := DB.Model(&result).
		Column("id", "name", "full_name", "htmlurl", "stargazers_count", "forks_count", "description", "avatar_url").
		Where("id = ?", id).
		Order("stargazers_count DESC NULLS LAST").
		Select()

	if err != nil {
		return ""
	}

	result.Modules = queryModules(result.ID, level)
	j, _ := json.MarshalIndent(result, "", "  ")

	var buf bytes.Buffer
	gzipErr := utils.Gzip(&buf, j)
	if gzipErr != nil {
		log.Println(gzipErr)
	}

	key := fmt.Sprintf("%s-%s", id, l)
	redisClient.Set(key, buf.Bytes(), time.Minute*30).Result()

	return string(j)
}

func getQueryString(id int) string {
	return fmt.Sprintf(`
		SELECT "repo"."id", "repo"."name", "repo"."full_name", "repo"."stargazers_count", "repo"."forks_count", "repo"."avatar_url", "repo"."description"
		FROM "repos" as "repo"
		JOIN  "repo_to_repos" ON "repo"."id" = "repo_to_repos"."module_id"
		WHERE ("repo_to_repos"."module_id" = "repo"."id") AND ("repo_to_repos"."repo_id"=%v)
		ORDER BY "repo"."stargazers_count" DESC NULLS LAST;
	`, id)
}

func appendPointers(modules []Repo) []*Repo {
	p := []*Repo{}
	for i := range modules {
		p = append(p, &modules[i])
	}

	return p
}

func queryModules(id, level int) []Repo {
	modules := []Repo{}

	query := getQueryString(id)

	_, err := DB.Model(&modules).Query(&modules, query)
	if err != nil {
		fmt.Println(err)
	}

	if level > 1 {
		if level > 5 {
			level = 5
		}

		modulesPts := appendPointers(modules)
		for level > 1 {
			pts := []*Repo{}

			for len(modulesPts) > 0 {

				child := modulesPts[0]
				childModules := []Repo{}

				q := getQueryString(child.ID)
				DB.Model(&childModules).Query(&childModules, q)

				child.Modules = childModules
				pts = append(pts, appendPointers(childModules)...)

				modulesPts = modulesPts[1:]
			}

			modulesPts = pts
			level--
		}
	}

	return modules
}

// SelectMultipleByID selects multuple repos
func SelectMultipleByID(ids string) string {
	idsStr := strings.Split(ids, ",")
	result := []Repo{}

	for _, id := range idsStr {
		idInt, err := utils.StrToInt(id)
		if err != nil {
			continue
		}
		repo := new(Repo)
		err = DB.Model(repo).
			Column("id", "name", "full_name", "htmlurl", "stargazers_count", "forks_count", "description").
			Where("id = ?", id).
			Order("stargazers_count DESC NULLS LAST").
			Select()

		if err != nil {
			continue
		}

		repo.Modules = queryModules(idInt, 1)
		result = append(result, *repo)
	}

	dbResponse := DBResponse{
		Count: len(result),
		Items: result,
	}

	j, _ := json.MarshalIndent(dbResponse, "", "  ")

	return string(j)
}

// SelectReadme selects readme
func SelectReadme(id string) string {
	var readme string

	err := DB.Model((*Repo)(nil)).
		Column("readme").
		Where("id = ?", id).
		Select(&readme)
	if err != nil {
		fmt.Println(err)
	}

	return readme
}

// Search searchs if name or full_name or description contains search term
func Search(term string) []byte {
	var repos []Repo
	fmt.Println(term, "SEARCH")
	term = "%" + strings.ToLower(term) + "%"
	titleTerm := strings.ToTitle(term) + "%"
	err := DB.Model(&repos).
		Column("id", "full_name", "avatar_url", "stargazers_count", "forks_count", "description").
		Where("name like ?", term).
		WhereOr("full_name like ?", term).
		WhereOr("description like ?", term).
		WhereOr("description like ?", titleTerm).
		Order("stargazers_count DESC NULLS LAST").
		Limit(50).
		Select()

	utils.HandleErrEXIT(err, "SEARCH")

	dbResponse := DBResponse{
		Count: len(repos),
		Items: repos,
	}

	j, _ := json.MarshalIndent(dbResponse, "", "  ")

	return j
	// return string(j), nil
}
