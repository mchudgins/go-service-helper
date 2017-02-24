package actuator

// actuator provides a basic mux which will let a configurable endpoint/url
// query for the valid url's of the service.

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"text/template"
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
`

	json = `
`

	text = `
`
)

var (
	htmlTemplate = template.Must(template.New("html").Parse(html))
	jsonTemplate = template.Must(template.New("json").Parse(json))
	textTemplate = template.Must(template.New("text").Parse(text))
)

func init() {

}

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
	if strings.Compare(m.mappingURL, r.URL.Path) == 0 {
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

func (m *ActuatorMux) displayEndpoints(w http.ResponseWriter, r *http.Request) {
	if m.fDirty {
		m.mutex.Lock()
		sort.Strings(m.mappings)
		m.fDirty = false
		m.mutex.Unlock()
	}

}
