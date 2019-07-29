package database

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/a-kairat/go-repos-api/structs"
	"github.com/a-kairat/go-repos-api/utils"

	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
)

type Repo struct {
	ID              int
	Name            string `json:"name" sql:",nullable"`
	FullName        string `json:"full_name" sql:",unique"`
	HTMLURL         string `json:"html_url" sql:",nullable"`
	Description     string `json:"description" sql:",nullable"`
	StargazersCount int    `json:"stargazers_count" sql:",nullable"`
	ForksCount      int    `json:"forks_count" sql:",nullable"`
	AvatarURL       string `json:"avatar_url" sql:",nullable"`
	Modules         []Repo `json:"modules" pg:"many2many:repo_to_repos,joinFK:module_id,zeroable"`
}

type RepoToRepos struct {
	RepoID   int
	ModuleID int
}

type DBResponse struct {
	Count int
	Items []Repo
}

func init() {
	// Register many to many model so ORM can better recognize m2m relation.
	// This should be done before dependant models are used.
	orm.RegisterTable((*RepoToRepos)(nil))
}

func Connect() *pg.DB {
	db := pg.Connect(&pg.Options{
		User:     "kairat",   // os.LookupEnv("DBUSER")
		Password: "psqlpswd", // os.LookupEnv("DBPASSWORD")
	})

	return db
}

func CreateSchema(db *pg.DB) error {
	models := []interface{}{
		(*Repo)(nil),
		(*RepoToRepos)(nil),
	}
	for _, model := range models {
		err := db.CreateTable(model, &orm.CreateTableOptions{
			// Temp: true,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func Insert(db *pg.DB, v structs.Item) {
	repo := &Repo{
		Name:            v.Name,
		FullName:        v.FullName,
		HTMLURL:         v.HTMLURL,
		Description:     v.Description,
		StargazersCount: v.StargazersCount,
		ForksCount:      v.ForksCount,
		AvatarURL:       v.Owner.AvatarURL,
	}

	_, err := db.Model(repo).
		OnConflict("(full_name) DO UPDATE").
		// Set("id = EXCLUDED.id").
		Insert()
	utils.HandleErrPanic(err, "DB REPO INSERT")

	for _, mod := range v.Modules {

		module := &Repo{
			Name:            mod.Name,
			FullName:        mod.FullName,
			HTMLURL:         mod.HTMLURL,
			Description:     mod.Description,
			StargazersCount: mod.StargazersCount,
			ForksCount:      mod.ForksCount,
			AvatarURL:       mod.Owner.AvatarURL,
		}

		_, err := db.Model(module).
			OnConflict("(full_name) DO UPDATE").
			// Set("id = EXCLUDED.id").
			Insert()

		utils.HandleErrPanic(err, "DB MODULE INSERT")

		repoToModule := &RepoToRepos{RepoID: repo.ID, ModuleID: module.ID}

		resp, err := db.Model(repoToModule).
			Where("repo_id = ?repo_id").
			Where("module_id = ?module_id").
			SelectOrInsert()

		utils.HandleErrPanic(err, "DB REPO TO REPO SELECT OR INSERT")

		fmt.Println(repo.ID, module.ID, resp)

	}

}

func Select(db *pg.DB, name string) {
	var repos []Repo
	// repo := new(Repo)
	err := db.Model(repos).
		Relation("Modules", func(q *orm.Query) (*orm.Query, error) {
			q = q.OrderExpr("repo.id DESC")
			return q, nil
		}).
		Where("name = ?", name).
		Select()
	utils.HandleErrPanic(err, "SELECT WITH RELATION")

	j, jErr := json.MarshalIndent(repos, "", " ")

	utils.HandleErrPanic(jErr, "JSON WITH INDENT")

	fmt.Println(string(j))
}

func SelectALL(db *pg.DB, name string, repos *[]Repo) {

	err := db.Model(repos).
		Where("name = ?", name).
		Order("stargazers_count DESC NULLS LAST").
		Select()

	utils.HandleErrPanic(err, "SELECT ALL")

}

func SelectLimitOffset(db *pg.DB, page string) ([]Repo, error) {
	limit := 10
	p, _ := utils.StrToInt(page)
	offset := limit * (p - 1)

	var repos []Repo

	err := db.Model(&repos).
		Order("stargazers_count DESC NULLS LAST").
		Limit(limit).
		Offset(offset).
		Select()

	utils.HandleErrPanic(err, "SELECT WITH LIMIT AND OFFSET")

	return repos, nil
}

func SelectModule(db *pg.DB, name, level string) (string, error) {

	var repos []Repo
	// repo := new(Repo)
	if level == "" {
		level = "1"
	}

	rLevel, lErr := utils.StrToInt(level)
	if lErr != nil {
		return "", fmt.Errorf("Recursion level")
	}

	SelectALL(db, name, &repos)
	repos, reposErr := getRecursiveModulesForEach(db, repos, rLevel)

	utils.HandleErrPanic(reposErr, "GET MODULES FOR EACH")

	dbResponse := DBResponse{
		Count: len(repos),
		Items: repos,
	}

	j, _ := json.MarshalIndent(dbResponse, "", "  ")

	return string(j), nil
}

func getRecursiveModulesForEach(db *pg.DB, repos []Repo, level int) ([]Repo, error) {
	if level > 6 {
		level = 6
	}

	if level > 0 {
		for i, repo := range repos {
			r, qErr := queryModules(db, repo.FullName, repo)
			if qErr != nil {
				return []Repo{}, fmt.Errorf("Item not found")
			}
			repos[i] = r
		}
		level--
		for i, repo := range repos {
			r, err := getRecursiveModulesForEach(db, repo.Modules, level)
			if err != nil {
				continue
			}
			repos[i].Modules = r
		}
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
		return Repo{}, fmt.Errorf("Item not found")
	}

	return repo, nil
}

func Search(db *pg.DB, term string) (string, error) {
	var repos []Repo

	term = "%" + strings.ToLower(term) + "%"

	err := db.Model(&repos).
		Where("name like ?", term).
		WhereOr("name like ?", strings.Title(term)).
		WhereOr("name like ?", strings.ToUpper(term)).
		WhereOr("full_name like ?", term).
		WhereOr("full_name like ?", strings.Title(term)).
		WhereOr("full_name like ?", strings.ToUpper(term)).
		WhereOr("description like ?", term).
		WhereOr("description like ?", strings.Title(term)).
		WhereOr("description like ?", strings.ToUpper(term)).
		Order("stargazers_count DESC NULLS LAST").
		Limit(50).
		Select()

	utils.HandleErrPanic(err, "SEARCH")

	dbResponse := DBResponse{
		Count: len(repos),
		Items: repos,
	}

	j, _ := json.MarshalIndent(dbResponse, "", "  ")

	return string(j), nil
}
