package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/kataras/compress"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/readwrite", readWrite)
	mux.HandleFunc("/info", printInfo)

	// Wrap any handler with the compress.Handler to enable
	// writing and reading compressed data.
	// At this example we are wrapping the entire router.
	log.Println("Server is listening at :8080")
	http.ListenAndServe(":8080", compress.Handler(mux))
}

type payload struct {
	Data string `json:"data"`
}

// Zero changes on your handlers, write them as usual, e.g.
func readWrite(w http.ResponseWriter, r *http.Request) {
	var p payload
	// Reads using comrpession.
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		log.Printf("handler: json decoder: %v", err)

		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, http.StatusText(http.StatusBadRequest))
		return
	}

	// Writes using compression.
	fmt.Fprintf(w, "Data received: %s", p.Data)
}

// Check if writer and reader are using compression.
func printInfo(w http.ResponseWriter, r *http.Request) {
	if cw, ok := w.(*compress.ResponseWriter); ok {
		// If request accepts a valid encoding then this will be executed:
		log.Printf("Writing using '%s' encoding and %d compression level.", cw.Encoding, cw.Level)
	}

	if cr, ok := r.Body.(*compress.Reader); ok {
		// If body has data and are compressed
		// then this will be executed too:
		log.Printf("Reading using '%s' encoding.", cr.Encoding)
	}
}
