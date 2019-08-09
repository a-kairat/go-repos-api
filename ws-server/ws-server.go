package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"

	database "github.com/a-sube/go-repos-api/db"
	"github.com/a-sube/go-repos-api/utils"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var (
	origin, originOK = os.LookupEnv("ORIGIN") // depends
	upgrader         = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,

		// CheckOrigin returns true if the request Origin header is acceptable. If
		// CheckOrigin is nil, then a safe default is used: return false if the
		// Origin request header is present and the origin host is not equal to
		// request Host header.
		//
		// A CheckOrigin function should carefully validate the request origin to
		// prevent cross-site request forgery.
		CheckOrigin: func(r *http.Request) bool {

			if !originOK {
				log.Println("ORIGIN for ws server is not set")
				os.Exit(1)
			}

			// the most simple check origin
			if r.Header.Get("Origin") == origin {
				return true
			}

			return false
		},
	}
)

func main() {

	utils.CheckEnvVars(true, true, false)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	go func() {
		s := <-sigs
		log.Printf("RECEIVED SIGNAL: %s", s)
		os.Exit(1)
	}()

	router := mux.NewRouter()
	router.HandleFunc("/ws", search)

	http.Handle("/", router)

	log.Fatal(http.ListenAndServe(":3005", router))
}

func search(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Println(err)
		return
	}

	go handleConn(conn)
}

func handleConn(conn *websocket.Conn) {
	defer conn.Close()

	for {
		msgType, msg, readErr := conn.ReadMessage()
		if readErr != nil {
			log.Println(readErr)
			return
		}

		if string(msg) != "ping" {
			data := database.Search(string(msg))
			if connErr := conn.WriteMessage(msgType, data); connErr != nil {
				return
			}
		}
	}
}
