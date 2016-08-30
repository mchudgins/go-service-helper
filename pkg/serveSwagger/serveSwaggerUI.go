package serveSwagger

import (
	"log"
	"net/http"
)

// TODO: this package should be refactored to use either go-bindata-assetfs OR
// https://github.com/GeertJohan/go.rice

// This package serves up the Swagger UI at the designated path
// example:
//  		swaggerProxy, _ := serveSwagger.NewSwaggerProxy("/swagger-ui/")
//  		http.HandleFunc("/swagger-ui/", swaggerProxy.ServeHTTP)
//      http.ListenAndServe(":8080", nil)

// SwaggerProxy serves the swagger UI at the designated path
type SwaggerProxy struct {
	path    string
	pathLen int
	h       http.Handler
}

// NewSwaggerProxy initializes the SwaggerProxy struct
func NewSwaggerProxy(proxyPath string) (*SwaggerProxy, error) {
	return &SwaggerProxy{path: proxyPath,
			pathLen: len(proxyPath),
			//		h:       http.FileServer(http.Dir("/home/mchudgins/golang/src/github.com/swagger-api/swagger-ui/dist"))}, nil
			h: http.FileServer(assetFS())},
		nil
}

// ServeHTTP serves up the Swagger UI
func (s *SwaggerProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	log.Printf("Query: %s", r.URL.RawQuery)
	path := r.URL.Path[s.pathLen:]
	if len(path) == 0 && len(r.URL.RawQuery) == 0 {
		log.Printf("hmmm. redirecting.  path: %s; query: %s; query len: %d", path, r.URL.RawQuery, len(r.URL.RawQuery))
		http.Redirect(w, r,
			"/swagger-ui/?url=/swagger/service.swagger.json", http.StatusMovedPermanently)
		return
	}

	if len(path) == 0 {
		r.URL.Path = "/"
	} else {
		r.URL.Path = path
	}

	log.Printf("path: %s", r.URL.Path)

	s.h.ServeHTTP(w, r)
}
