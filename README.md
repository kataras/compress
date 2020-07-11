# Compress

[![build status](https://img.shields.io/travis/com/kataras/compress/master.svg?style=for-the-badge&logo=travis)](https://travis-ci.com/github/kataras/compress) [![report card](https://img.shields.io/badge/report%20card-a%2B-ff3333.svg?style=for-the-badge)](https://goreportcard.com/report/github.com/kataras/compress) [![godocs](https://img.shields.io/badge/go-%20docs-488AC7.svg?style=for-the-badge)](https://godoc.org/github.com/kataras/compress)

Fast and easy-to-use compression package for Go applications.

## Installation

The only requirement is the [Go Programming Language](https://golang.org/dl).

```sh
$ go get github.com/kataras/compress
```

## Getting Started

Import the package:

```go
package main

import "github.com/kataras/compress"
```

Wrap a handler to enable writing and reading using the best offered compression:

```go
import "net/http"

mux := http.NewServeMux()
// [...]
http.ListenAndServe(":8080", compress.Handler(mux))
```

Wrap any `io.Writer` for writing data using compression with `NewWriter`:

```go
import "bytes"
import "encoding/json"

buf := new(bytes.Buffer)

w, err := compress.NewWriter(buf, compress.GZIP, -1)
if err != nil {
    panic(err)
}

json.NewEncoder(w).Encode(payload{Data: "my data"})

w.Close()
```

Wrap any `io.Reader` for reading compressed data with `NewReader`:

```go
// Where resp.Body is an io.Reader.
r, err := compress.NewReader(resp.Body, compress.GZIP)
if err != nil {
    panic(err)
}
defer r.Close()

body, err := ioutil.ReadAll(r)
```

Supported compression algorithms:

- gzip
- deflate
- brotli
- snappy

Please navigate through [_examples](_examples) directory for more.

## License

This software is licensed under the [MIT License](LICENSE).
