package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/kataras/compress"
)

const baseURL = "http://localhost:8080"

// Available options:
// - "gzip",
// - "deflate",
// - "br" (for brotli),
// - "snappy" and
// - "s2"
const encoding = compress.BROTLI

var client = http.DefaultClient

func main() {
	fmt.Printf("Server should be running at: %s\n", baseURL)
	runExample()
}

type payload struct {
	Data string `json:"data"`
}

func runExample() {
	buf := new(bytes.Buffer)

	// Compress client's data.
	cw, err := compress.NewWriter(buf, encoding, -1)
	if err != nil {
		panic(err)
	}

	json.NewEncoder(cw).Encode(payload{Data: "my data"})

	// `Close` or `Flush` required before `NewRequest` call.
	cw.Close()

	endpoint := baseURL + "/readwrite"

	req, err := http.NewRequest(http.MethodPost, endpoint, buf)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Required to send gzip compressed data to the server.
	req.Header.Set("Content-Encoding", encoding)
	// Required to receive server's compressed data.
	req.Header.Set("Accept-Encoding", encoding)

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Decompress server's compressed reply.
	cr, err := compress.NewReader(resp.Body, encoding)
	if err != nil {
		panic(err)
	}
	defer cr.Close()

	body, err := ioutil.ReadAll(cr)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Server replied with: %s", string(body))
}
