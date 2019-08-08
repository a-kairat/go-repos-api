package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/a-sube/go-repos-api/utils"

	database "github.com/a-sube/go-repos-api/db"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

var (
	//db           = database.Connect()
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	c = cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})
)

func main() {

	utils.CheckEnvVars(true, true, false)

	router := mux.NewRouter()

	router.HandleFunc("/page/", page)     // /page/?page=<page>
	router.HandleFunc("/module/", module) // /module/?name=<name> or /module/?id=<id>

	router.HandleFunc("/search/", search) // /search/?search=<term>
	router.HandleFunc("/multi/", multi)   // /multi/?ids=1,2,3,4,5
	router.HandleFunc("/readme/", readme)
	http.Handle("/", router)

	var servers []*http.Server
	for i := 0; i < 4; i++ {
		addr := fmt.Sprintf("127.0.0.1:300" + utils.IntToStr(i))
		srv := &http.Server{
			Addr:         addr,
			WriteTimeout: time.Second * 15,
			ReadTimeout:  time.Second * 15,
			IdleTimeout:  time.Second * 60,
			Handler:      router, // instance of gorilla/mux
		}
		servers = append(servers, srv)

		go func() {
			if err := srv.ListenAndServe(); err != nil {
				log.Println(err)
			}
		}()
	}

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, os.Interrupt)

	<-sigs
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	for _, server := range servers {
		server.Shutdown(ctx)
	}
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	log.Println("shutting down")
	os.Exit(0)
}

func page(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)

	page := r.URL.Query().Get("page")
	limit := r.URL.Query().Get("limit")

	if page == "" {
		page = "1"
	}

	resp, err := database.SelectLimitOffset(page, limit)
	utils.HandleErrLog(err, "PAGE FUNC: DB ERROR")

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(resp)
	utils.HandleErrLog(err, "PAGE FUNC: OK")
	return
}

func module(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)

	name := r.URL.Query().Get("name")
	id := r.URL.Query().Get("id")

	if name != "" {
		result := database.SelectALLByName(name)

		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintf(w, result)
		utils.HandleErrLog(err, "MODULE FUNC: OK - with name param")
		return
	}

	if id != "" {
		depthLevel := r.URL.Query().Get("depth")

		if depthLevel != "" {
			key := fmt.Sprintf("%s-%s", id, depthLevel)

			var byteResult []byte
			var result string
			byteResult, redisErr := redisClient.Get(key).Bytes()

			if redisErr != nil {
				result = database.SelectByIDWithModules(id, depthLevel)
			} else {
				var buf bytes.Buffer
				utils.Ungzip(&buf, byteResult)
				result = buf.String()
			}

			if result != "" {
				w.WriteHeader(http.StatusOK)
				_, err := fmt.Fprintf(w, result)
				utils.HandleErrLog(err, "MODULE FUNC: OK - with depth")
				return
			}

			w.WriteHeader(http.StatusNotFound)
			_, err := fmt.Fprintf(w, "Invalid recursion level provided")
			utils.HandleErrLog(err, "MODULE FUNC: NOT FOUND - with depth")
			return
		}

		result := database.SelectByID(id)
		if result != "" {
			w.WriteHeader(http.StatusOK)
			_, err := fmt.Fprintf(w, result)
			utils.HandleErrLog(err, "MODULE FUNC: OK - select by ID")
			return
		}

		w.WriteHeader(http.StatusNotFound)
		_, err := fmt.Fprintf(w, "Not Found")
		utils.HandleErrLog(err, "MODULE FUNC: NOT FOUND - with id param")
		return
	}

	w.WriteHeader(http.StatusNotFound)
	_, err := fmt.Fprintf(w, "'id' or 'name' parameters required. Example URL /module/?id=<id> or /module/?name=<name>")
	utils.HandleErrLog(err, "MODULE FUNC: NOT FOUND - with name param")
	return
}

func search(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)

	term := r.URL.Query().Get("search")
	if term != "" {
		repo := database.Search(term)
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintf(w, string(repo))
		utils.HandleErrLog(err, "SEARCH FUNC: OK")
		return
	}

	w.WriteHeader(http.StatusNotFound)
	_, err := fmt.Fprintf(w, "Invalid search request")
	utils.HandleErrLog(err, "SEARCH FUNC: NOT FOUND")
}

func multi(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	ids := r.URL.Query().Get("ids")

	if ids != "" {
		resp := database.SelectMultipleByID(ids)

		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintf(w, resp)
		utils.HandleErrLog(err, "MULTI FUNC: OK")
		return
	}

	w.WriteHeader(http.StatusNotFound)
	_, err := fmt.Fprintf(w, "Fail multi request")
	utils.HandleErrLog(err, "MULTI FUNC: NOT FOUND")
}

func readme(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	id := r.URL.Query().Get("id")
	if id != "" {
		resp := database.SelectReadme(id)

		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(resp)
		utils.HandleErrLog(err, "README FUNC: JSON ENCODE")
		return
	}

	w.WriteHeader(http.StatusNotFound)
	_, err := fmt.Fprintf(w, "Fail readme request")
	utils.HandleErrLog(err, "README FUNC: NOT FOUND")
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}
