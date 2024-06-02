package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func jsonPrint(data interface{}) {
	val, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		log.Fatal("Failed to marshal value")
	}

	fmt.Println(string(val))
}

func main() {
	r := chi.NewRouter()

	r.Get("/posts", func(w http.ResponseWriter, r *http.Request) {
		jsonPrint(r.Header)
	})

	http.ListenAndServe(":8000", r)
}
