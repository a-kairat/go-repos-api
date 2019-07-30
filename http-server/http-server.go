package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	database "github.com/a-sube/go-repos-api/db"
	"github.com/go-redis/redis"

	"github.com/gorilla/mux"
)

var (
	db          = database.Connect()
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
)

func main() {

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs)

	go func() {
		s := <-sigs
		log.Printf("RECEIVED SIGNAL: %s", s)
		os.Exit(1)
	}()

	router := mux.NewRouter()

	router.HandleFunc("/", root)
	router.HandleFunc("/module/{module}", module)
	router.HandleFunc("/search/{term}", search)

	http.Handle("/", router)

	log.Fatal(http.ListenAndServe(":3000", router))
}

func root(w http.ResponseWriter, r *http.Request) {

	page := r.URL.Query().Get("page")
	if page == "" {
		page = "1"
	}

	resp, err := database.SelectLimitOffset(db, page)

	if err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
	return
}

func module(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	recursive := r.URL.Query().Get("recursive")
	if val, ok := vars["module"]; ok {

		key := recursive + val
		cached, err := redisClient.Get(key).Result()

		if err != nil {
			repo, err := database.SelectModule(db, val, recursive)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "Error: Module not found")
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, repo)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, cached)
		return

	}

	w.WriteHeader(http.StatusNotFound)
}

func search(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	if val, ok := vars["term"]; ok {

		repo, err := database.Search(db, val)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "Error: Module not found")
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, repo)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}
