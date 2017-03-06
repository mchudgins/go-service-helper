package actuator

import "net/http"

type MVCMux struct {
	handler http.Handler
}

type FrontController interface {

}

type Provider interface {
	ProcessRequest(r *http.Request) (viewName string, model interface{}, status int, err error)
}

type Viewer interface {
	CanHandle(mediaType string, model interface{}) bool
	View(model interface{}) ([]byte, error)
}

func NewMVCMux(h http.Handler) *MVCMux {
	if h == nil {
		return NewMVCMux(NewActuatorMux(""))
	}
	mvcMux := &MVCMux{
		handler: h,
	}

	return mvcMux
}

func (m *MVCMux) AddProvider(pattern string, p Provider) {

}

func (m *MVCMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.
	m.handler.ServeHTTP(w, r)
}
