package main

import (
	"bytes"
	"net/http"
)

type interceptingWriter struct {
	Headers        http.Header
	body           *bytes.Buffer
	statusCode     int
	responseWriter http.ResponseWriter
}

func (r *interceptingWriter) Header() http.Header {
	if r.Headers == nil {
		r.Headers = make(http.Header)
	}
	return r.Headers
}

func (r *interceptingWriter) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func (r *interceptingWriter) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func NotFoundHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &interceptingWriter{body: &bytes.Buffer{}, responseWriter: w}
		next.ServeHTTP(writer, r)
		if writer.statusCode == http.StatusNotFound || (writer.statusCode == 0 && writer.body.String() == "404 page not found\n") {
			http.ServeFile(w, r, "layout/404.html")
		} else {
			for k, vs := range writer.Headers {
				for _, v := range vs {
					w.Header().Set(k, v)
				}
			}
			if writer.statusCode != 0 {
				w.WriteHeader(writer.statusCode)
			}
			_, _ = w.Write(writer.body.Bytes())
		}
	})
}
