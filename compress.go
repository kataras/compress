package compress

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	// Pick the fastest compression packages for the job.
	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/s2" // Snappy output but likely faster decompression.
	"github.com/klauspost/compress/snappy"
)

// The available builtin compression algorithms.
const (
	GZIP    = "gzip"
	DEFLATE = "deflate"
	BROTLI  = "br"
	SNAPPY  = "snappy"
	S2      = "s2"

	// IDENTITY when no transformation whatsoever.
	IDENTITY = "identity"
)

var (
	// ErrResponseNotCompressed returned from NewResponseWriter
	// when response's Content-Type header is missing due to golang/go/issues/31753 or
	// when accept-encoding is empty. The caller should fallback to the original response writer.
	ErrResponseNotCompressed = errors.New("compress: response will not be compressed")
	// ErrRequestNotCompressed returned from NewReader
	// when request is not compressed.
	ErrRequestNotCompressed = errors.New("compress: request is not compressed")
	// ErrNotSupportedCompression returned from
	// NewResponseWriter, NewWriter and NewReader
	// when the request's Accept-Encoding was not found in the server's supported
	// compression algorithms. Check that error with `errors.Is`.
	ErrNotSupportedCompression = errors.New("compress: unsupported compression")
)

// Writer is an interface which all compress writers should implement.
type Writer interface {
	io.WriteCloser
	// All known implementations contain `Flush`, `Reset` (and `Close`) methods,
	// so we wanna declare them upfront.
	Flush() error
	Reset(io.Writer)
}

// NewWriter returns a Writer of "w" based on the given "encoding".
func NewWriter(w io.Writer, encoding string, level int) (cw Writer, err error) {
	switch encoding {
	case GZIP:
		cw, err = gzip.NewWriterLevel(w, level)
	case DEFLATE: // -1 default level, same for gzip.
		cw, err = flate.NewWriter(w, level)
	case BROTLI: // 6 default level.
		if level == -1 {
			level = 6
		}
		cw = brotli.NewWriterLevel(w, level)
	case SNAPPY:
		cw = snappy.NewWriter(w)
	case S2:
		cw = s2.NewWriter(w)
	default:
		// Throw if "identity" is given. As this is not acceptable on "Content-Encoding" header.
		// Only Accept-Encoding (client) can use that; it means, no transformation whatsoever.
		err = ErrNotSupportedCompression
	}

	return
}

// Reader is a structure which wraps a compressed reader.
// It is used for determination across common request body and a compressed one.
type Reader struct {
	io.ReadCloser

	// We need this to reset the body to its original state, if requested.
	Src io.ReadCloser
	// Encoding is the compression alogirthm is used to decompress and read the data.
	Encoding string
}

// NewReader returns a new "Reader" wrapper of "src".
// It returns `ErrRequestNotCompressed` if client's request data are not compressed
// or `ErrNotSupportedCompression` if server missing the decompression algorithm.
// Note: on server-side the request body (src) will be closed automaticaly.
func NewReader(src io.Reader, encoding string) (*Reader, error) {
	if encoding == "" || src == nil {
		return nil, ErrRequestNotCompressed
	}

	var (
		rc  io.ReadCloser
		err error
	)

	switch encoding {
	case GZIP:
		rc, err = gzip.NewReader(src)
	case DEFLATE:
		rc = flate.NewReader(src)
	case BROTLI:
		rc = &noOpReadCloser{brotli.NewReader(src)}
	case SNAPPY:
		rc = &noOpReadCloser{snappy.NewReader(src)}
	case S2:
		rc = &noOpReadCloser{s2.NewReader(src)}
	default:
		err = ErrNotSupportedCompression
	}

	if err != nil {
		return nil, err
	}

	srcReadCloser, ok := src.(io.ReadCloser)
	if !ok {
		srcReadCloser = &noOpReadCloser{src}
	}

	v := &Reader{
		ReadCloser: rc,
		Src:        srcReadCloser,
		Encoding:   encoding,
	}

	return v, nil
}

// Header keys.
const (
	AcceptEncodingHeaderKey  = "Accept-Encoding"
	VaryHeaderKey            = "Vary"
	ContentEncodingHeaderKey = "Content-Encoding"
	ContentLengthHeaderKey   = "Content-Length"
	ContentTypeHeaderKey     = "Content-Type"
)

// AddCompressHeaders just adds the headers "Vary" to "Accept-Encoding"
// and "Content-Encoding" to the given encoding.
func AddCompressHeaders(h http.Header, encoding string) {
	h.Set(VaryHeaderKey, AcceptEncodingHeaderKey)
	h.Set(ContentEncodingHeaderKey, encoding)
}

// ResponseWriter is a compressed data http.ResponseWriter.
type ResponseWriter struct {
	Writer

	http.ResponseWriter
	http.Pusher
	http.CloseNotifier
	http.Hijacker

	Encoding  string
	Level     int
	AutoFlush bool // defaults to true, flushes buffered data on each Write.

	wroteHeader bool
}

var _ http.ResponseWriter = (*ResponseWriter)(nil)

// NewResponseWriter wraps the "w" response writer and
// returns a new compress response writer instance.
// It accepts http response writer, a net/http request value and
// the level of compression (use -1 for default compression level).
//
// It returns the best candidate among "gzip", "defate", "br", "snappy" and "s2"
// based on the request's "Accept-Encoding" header value.
//
// See `Handler/WriteHandler` for its usage. In-short, the caller should
// clear the writer through `defer Close()`.
func NewResponseWriter(w http.ResponseWriter, r *http.Request, level int) (*ResponseWriter, error) {
	acceptEncoding := r.Header.Values(AcceptEncodingHeaderKey)

	if len(acceptEncoding) == 0 {
		return nil, ErrResponseNotCompressed
	}

	encoding := negotiateAcceptHeader(acceptEncoding, []string{GZIP, DEFLATE, BROTLI, SNAPPY, S2}, IDENTITY)
	if encoding == "" {
		return nil, fmt.Errorf("%w: %s", ErrNotSupportedCompression, encoding)
	}

	if level == -1 && encoding == BROTLI {
		level = 6
	}

	cr, err := NewWriter(w, encoding, level)
	if err != nil {
		return nil, err
	}

	AddCompressHeaders(w.Header(), encoding)

	pusher, ok := w.(http.Pusher)
	if !ok {
		pusher = nil // make sure interface value is nil.
	}

	// This interface is obselete by Go authors
	// and we only capture it
	// for compatible reasons. End-developers SHOULD replace
	// the use of CloseNotifier with the: Request.Context().Done() channel.
	closeNotifier, ok := w.(http.CloseNotifier)
	if !ok {
		closeNotifier = nil
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		hijacker = nil
	}

	v := &ResponseWriter{
		ResponseWriter: w,
		Pusher:         pusher,
		CloseNotifier:  closeNotifier,
		Hijacker:       hijacker,
		Level:          level,
		Encoding:       encoding,
		Writer:         cr,
		AutoFlush:      true,
	}

	return v, nil
}

func (w *ResponseWriter) Write(p []byte) (int, error) {
	h := w.Header()
	if _, has := h[ContentTypeHeaderKey]; !has {
		h[ContentTypeHeaderKey] = []string{http.DetectContentType(p)}
	}

	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	n, err := w.Writer.Write(p)
	if err != nil {
		return 0, err
	}

	if w.AutoFlush {
		err = w.Writer.Flush()
	}

	return n, err
}

// WriteHeader sends an HTTP response header with the provided
// status code. Deletes the "Content-Length" response header and
// calls the ResponseWriter's WriteHeader method.
func (w *ResponseWriter) WriteHeader(statusCode int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		delete(w.Header(), ContentLengthHeaderKey)

		w.ResponseWriter.WriteHeader(statusCode)
	}
}

// Flush sends any buffered data to the client.
func (w *ResponseWriter) Flush() {
	w.Writer.Flush()

	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

type (
	noOpWriter struct{}

	noOpReadCloser struct {
		io.Reader
	}
)

var (
	_ io.ReadCloser = (*noOpReadCloser)(nil)
	_ io.Writer     = (*noOpWriter)(nil)
)

func (w *noOpWriter) Write(p []byte) (int, error) { return 0, nil }

func (r *noOpReadCloser) Close() error {
	return nil
}
