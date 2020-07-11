package compress

import "net/http"

// Handler wraps a Handler and returns a new one
// which makes future Write calls to compress the data before sent
// and future request body to decompress the incoming data before read.
func Handler(next http.Handler) http.HandlerFunc {
	return WriteHandler(ReadHandler(next))
}

// WriteHandler is the write using compression middleware.
func WriteHandler(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cr, err := NewResponseWriter(w, r, -1)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		defer cr.Close()

		r.Header.Del(AcceptEncodingHeaderKey)
		next.ServeHTTP(cr, r)
	}
}

// ReadHandler is the decompress and read request body middleware.
func ReadHandler(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		encoding := r.Header.Get(ContentEncodingHeaderKey)
		if encoding != "" {
			rc, err := NewReader(r.Body, encoding)
			if err == nil {
				defer rc.Close()
				r.Body = rc
			}
		}

		next.ServeHTTP(w, r)
	}
}
