package actuator

import "net/http"

type MVCMux struct {
	*ActuatorMux
}

type Provider interface {
	ProcessRequest(r *http.Request) (viewName string, model interface{})
}

type Viewer interface {
	CanHandle(mediaType string, model interface{}) bool
	View(model interface{}) ([]byte, error)
}

func NewMVCMux() *MVCMux {
	mvcMux := &MVCMux{
		ActuatorMux: NewActuatorMux(""),
	}

	return mvcMux
}

func (m *MVCMux) AddProvider(pattern string, p Provider) {

}

func (m *MVCMux) Handle(pattern string, handler http.Handler) {
	m.ServeMux.Handle(pattern, handler)
}

func (m *MVCMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.ServeMux.HandleFunc(pattern, handler)
}

func (m *MVCMux) Handler(r *http.Request) (h http.Handler, pattern string) {
	return m.ServeMux.Handler(r)
}

func (m *MVCMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.ServeMux.ServeHTTP(w, r)
}
