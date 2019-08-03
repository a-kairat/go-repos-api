package database

import "testing"

func TestConnect(t *testing.T) {

}

// CreateSchema creates Repo and RepoToRepo tables if not exists.
func TestCreateSchema(t *testing.T) {

}

// Insert takes `Item` struct, inserts it to Repo table,
// iterates over child modules and inserts each module it to RepoToRepos table.
func TestInsert(t *testing.T) {

}

// SelectALL selects all reposritories from table that have name = name.
func TestSelectALL(t *testing.T) {

}

// SelectLimitOffset is a paginator. Selects limited items per page.
// Default limit is 10.
func TestSelectLimitOffset(t *testing.T) {

}

// SelectModule selects single module and its child modules.
// Uses recursion to query modules for child modules.
func TestSelectModule(t *testing.T) {

}

func TestGetRecursiveModulesForEach(t *testing.T) {

}

func TestQueryModules(t *testing.T) {

}

// Search searchs if name or full_name or description contains search term
func TestSearch(t *testing.T) {

}
