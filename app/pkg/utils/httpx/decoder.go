package httpx

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func DecompressResponseBody(response *http.Response) (reader io.Reader, cleanup func(), err error) {
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		cleanup = func() { reader.(*gzip.Reader).Close() }
	case "deflate":
		reader = flate.NewReader(response.Body)
		cleanup = func() { reader.(io.ReadCloser).Close() } // FIXME: probably this io.ReadCloser.Close() does not work
	case "br":
		reader = brotli.NewReader(response.Body)
		cleanup = func() {}
	case "zstd":
		reader, err = zstd.NewReader(response.Body)
		cleanup = func() { reader.(*zstd.Decoder).Close() }
	default:
		reader = response.Body // No compression, use as is
		cleanup = func() { reader.(io.ReadCloser).Close() }
	}

	return reader, cleanup, err
}
