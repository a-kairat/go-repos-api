package database

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
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
func Connect() *pg.DB {

	db := pg.Connect(&pg.Options{
		User:     utils.DBUSER,
		Password: utils.DBPSWD,
	})

	return db
}

// CreateSchema creates Repo and RepoToRepo tables if not exists.
func CreateSchema(db *pg.DB) error {
	models := []interface{}{
		(*Repo)(nil),
		(*RepoToRepos)(nil),
	}
	for _, model := range models {
		err := db.CreateTable(model, &orm.CreateTableOptions{
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
func Insert(db *pg.DB, v structs.Item) {
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

	_, err := db.Model(repo).
		OnConflict("(full_name) DO UPDATE").
		// Set("id = EXCLUDED.id").
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

		_, err := db.Model(module).
			OnConflict("(full_name) DO UPDATE").
			// Set("id = EXCLUDED.id").
			Insert()

		utils.HandleErrEXIT(err, "DB MODULE INSERT")

		repoToModule := &RepoToRepos{RepoID: repo.ID, ModuleID: module.ID}

		_, err = db.Model(repoToModule).
			Where("repo_id = ?repo_id").
			Where("module_id = ?module_id").
			SelectOrInsert()

		utils.HandleErrEXIT(err, "DB REPO TO REPOS SELECT OR INSERT")

	}

}

// SelectALL selects all reposritories from table that have name = name.
func SelectALL(db *pg.DB, val, column string, repos *[]Repo) {
	var v interface{}
	if column == "id" {
		v, _ = utils.StrToInt(val)
	} else {
		v = val
	}
	column = fmt.Sprintf(column + " = ?")

	err := db.Model(repos).
		Where(column, v).
		Order("stargazers_count DESC NULLS LAST").
		Select()

	utils.HandleErrEXIT(err, "SELECT ALL")

}

// SelectLimitOffset is a paginator. Selects limited items per page.
// Default limit is 10.
func SelectLimitOffset(db *pg.DB, page, limit string) ([]Repo, error) {
	if limit == "" {
		limit = "10"
	}

	p, _ := utils.StrToInt(page)
	l, _ := utils.StrToInt(limit)
	offset := l * (p - 1)

	var repos []Repo

	err := db.Model(&repos).
		Order("stargazers_count DESC NULLS LAST").
		Limit(l).
		Offset(offset).
		Select()

	utils.HandleErrEXIT(err, "SELECT WITH LIMIT AND OFFSET")

	return repos, nil
}

// SelectModule selects single module and its child modules.
// Uses recursion to query modules for child modules.
func SelectModule(db *pg.DB, column, val, level string) (string, error) {

	var repos []Repo
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	fmt.Println(column, val)
	rLevel, _ := utils.StrToInt(level)

	SelectALL(db, val, column, &repos)

	repos, reposErr := getRecursiveModulesForEach(db, repos, rLevel)

	utils.HandleErrLog(reposErr, "GET MODULES FOR EACH")

	dbResponse := DBResponse{
		Count: len(repos),
		Items: repos,
	}

	j, _ := json.Marshal(dbResponse)

	if _, err := gz.Write(j); err != nil {
		panic(err)
	}
	fmt.Println(b.Len(), "BYTES")

	jString := string(j)

	key := level + val
	redisClient.Set(key, jString, time.Hour)

	return jString, nil
}

func getRecursiveModulesForEach(db *pg.DB, repos []Repo, level int) ([]Repo, error) {
	if level > 5 {
		level = 5
	}

	// recursion exit condition
	if level > 0 {
		// go over each repo in []Repo
		// query child modules for each of them
		for i, repo := range repos {
			r, qErr := queryModules(db, repo.FullName, repo)
			if qErr != nil {
				return []Repo{}, qErr
			}
			repos[i] = r
		}

		// go over each repo in []Repo
		// do the above operation for each repo in repo.Modules
		for i, repo := range repos {
			r, err := getRecursiveModulesForEach(db, repo.Modules, level)
			if err != nil {
				continue
			}
			repos[i].Modules = r
		}
		level--
	}
	return repos, nil
}

func queryModules(db *pg.DB, fullName string, repo Repo) (Repo, error) {
	err := db.Model(&repo).
		Relation("Modules", func(q *orm.Query) (*orm.Query, error) {
			q = q.OrderExpr("repo.stargazers_count DESC")
			return q, nil
		}).
		Where("full_name = ?", fullName).
		Select()

	if err != nil {
		return repo, fmt.Errorf("item not found %v", fullName)
	}

	return repo, nil
}

// Search searchs if name or full_name or description contains search term
func Search(db *pg.DB, term string) (string, error) {
	var repos []Repo

	term = "%" + strings.ToLower(term) + "%"
	titleTerm := strings.ToTitle(term) + "%"
	err := db.Model(&repos).
		Where("name like ?", term).
		// WhereOr("name like ?", strings.Title(term)).
		// WhereOr("name like ?", strings.ToUpper(term)).
		WhereOr("full_name like ?", term).
		// WhereOr("full_name like ?", strings.Title(term)).
		// WhereOr("full_name like ?", strings.ToUpper(term)).
		WhereOr("description like ?", term).
		WhereOr("description like ?", titleTerm).
		// WhereOr("description like ?", strings.ToUpper(term)).
		Order("stargazers_count DESC NULLS LAST").
		Limit(50).
		Select()

	utils.HandleErrEXIT(err, "SEARCH")

	dbResponse := DBResponse{
		Count: len(repos),
		Items: repos,
	}

	j, _ := json.MarshalIndent(dbResponse, "", "  ")

	return string(j), nil
}
