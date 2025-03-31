package serveSwagger

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

// This package serves up the Swagger UI at the designated path
// example:
//  	swaggerProxy, _ := serveSwagger.NewSwaggerProxy("/swagger-ui/")
//  	http.HandleFunc("/swagger-ui/", swaggerProxy.ServeHTTP)
//      http.ListenAndServe(":8080", nil)

// SwaggerProxy serves the swagger UI at the designated path
type SwaggerProxy struct {
	path    string
	pathLen int
	h       http.Handler
}

// NewSwaggerProxy initializes the SwaggerProxy struct
func NewSwaggerProxy(proxyPath string) (*SwaggerProxy, error) {
	return &SwaggerProxy{
			path:    proxyPath,
			pathLen: len(proxyPath),
			//h:       http.StripPrefix("/swagger-ui/", http.FileServer(http.Dir("/home/mchudgins/golang/src/github.com/swagger-api/swagger-ui/dist")))}, nil

			h: http.StripPrefix(proxyPath, Server)},
		nil

}

// ServeHTTP serves up the Swagger UI
func (s *SwaggerProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	log.WithField("path", r.URL.Path).WithField("proxyPath", s.path).Debug("ServeHTTP")

	if r.URL.Path == s.path {
		r.URL.Path = s.path + "/index.html"
		log.WithField("newPath", r.URL.Path).Debug("revising path")
	}

	s.h.ServeHTTP(w, r)
}
