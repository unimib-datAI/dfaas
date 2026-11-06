package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", Handle)

	log.Println("noop listening on :8090")
	log.Fatal(http.ListenAndServe(":8090", nil))
}

func Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("OK\n"))
}
