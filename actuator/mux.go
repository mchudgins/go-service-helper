package actuator

// actuator provides a basic mux which will let a configurable endpoint/url
// query for the valid url's of the service.

import (
	"encoding/json"
	"encoding/xml"
	template "html/template"
	"net/http"
	"os"
	"sort"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/golang/gddo/httputil/header"
)

type ActuatorMux struct {
	*http.ServeMux
	mappings   []string
	fDirty     bool
	mappingURL string
	mutex      *sync.Mutex
}

const (
	html = `
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="X-UA-Compatible" content="IE=edge,chrome=1">
  <title>Site Map</title>
</head>
<body>
<h1>Site Map</h1>
<ul>
{{range .Mappings}}<li><a href={{.}}>{{.}}</a></li>{{end}}
</ul>
</body>
</html>
`

	text = `
{{range .Mappings}}{{.}}
{{end}}
`
)

var (
	htmlTemplate = template.Must(template.New("html").Parse(html))
	textTemplate = template.Must(template.New("text").Parse(text))
)

// TODO: rename to NewServeMux() && add http.Handler as param
func NewActuatorMux(mappingURL string) *ActuatorMux {
	mux := http.NewServeMux()

	if len(mappingURL) == 0 {
		mappingURL = "/mappings"
	}

	actuator := &ActuatorMux{
		ServeMux:   mux,
		mappings:   []string{mappingURL},
		mappingURL: mappingURL,
		fDirty:     true,
		mutex:      &sync.Mutex{},
	}

	return actuator
}

func (m *ActuatorMux) Handle(pattern string, handler http.Handler) {
	m.ServeMux.Handle(pattern, handler)
	m.addMapping(pattern)
}

func (m *ActuatorMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.ServeMux.HandleFunc(pattern, handler)
	m.addMapping(pattern)
}

func (m *ActuatorMux) Handler(r *http.Request) (h http.Handler, pattern string) {
	m.addMapping(pattern)
	return m.ServeMux.Handler(r)
}

func (m *ActuatorMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.mappingURL == r.URL.Path {
		m.displayEndpoints(w, r)
	} else {
		m.ServeMux.ServeHTTP(w, r)
	}
}

func (m *ActuatorMux) addMapping(pattern string) {
	m.mutex.Lock()
	m.mappings = append(m.mappings, pattern)
	m.fDirty = true
	m.mutex.Unlock()
}

// display the endpoints
func (m *ActuatorMux) displayEndpoints(w http.ResponseWriter, r *http.Request) {
	if m.fDirty {
		m.mutex.Lock()
		sort.Strings(m.mappings)
		m.fDirty = false
		m.mutex.Unlock()
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	responseType := "text/text"
	view := textTemplate

	specs := header.ParseAccept(r.Header, "Accept")
	for _, spec := range specs {
		log.Debugf("spec Q: %f, type: %s", spec.Q, spec.Value)
		if spec.Q == 1.0 && spec.Value == "application/xml" {
			responseType = "application/xml"
			break
		}
		if spec.Q == 1.0 && spec.Value == "application/javascript" {
			responseType = "application/javascript"
			break
		}

		if spec.Q == 1.0 && spec.Value == "text/html" {
			responseType = "text/html"
			view = htmlTemplate
			break
		}
	}

	w.Header().Add("Content-Type", responseType)

	if responseType == "text/html" || responseType == "text/text" {
		type data struct {
			Hostname string
			URL      string
			Handler  string
			Mappings []string
		}

		err = view.Execute(w, data{Hostname: hostname,
			URL:      r.URL.Path,
			Handler:  "actuator.displayEndpoints",
			Mappings: m.mappings})
		if err != nil {
			log.WithError(err).Error("Unable to execute template")
		}
	} else {
		type response struct {
			Mappings []string `json:"siteMap" xml:"url"`
		}
		siteMap := &response{
			Mappings: m.mappings,
		}
		var out []byte
		var err error

		if responseType == "application/javascript" {
			out, err = json.Marshal(siteMap)

		}
		if responseType == "application/xml" {
			out, err = xml.Marshal(siteMap)
		}
		if err != nil {
			log.WithError(err).Fatal("unable to marshal site map")
		}
		w.Write(out)
	}
}
