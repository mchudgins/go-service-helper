package hystrix

import (
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/afex/hystrix-go/hystrix"
	"github.com/mchudgins/go-service-helper/httpWriter"
)

type hystrixHelper struct {
	commandName string
}

func NewHystrixHelper(commandName string) (*hystrixHelper, error) {
	hystrix.ConfigureCommand(commandName, hystrix.CommandConfig{
		MaxConcurrentRequests: 100,
	})
	return &hystrixHelper{commandName: commandName}, nil
}

func (y *hystrixHelper) Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := hystrix.Do(y.commandName, func() (err error) {

			monitor := httpWriter.NewHTTPWriter(w)

			h.ServeHTTP(monitor, r)

			rc := monitor.StatusCode()
			if rc >= 500 && rc < 600 {
				//log.Printf("StatusCode indicates backend failure")
				return fmt.Errorf("failure contacting %s", y.commandName)
			}
			return nil
		}, func(err error) error {
			log.WithError(err).WithField("hystrix command", y.commandName).Warn("breaker open")
			return nil
		})
		if err != nil {
			log.WithError(err).WithField("hystrix command", y.commandName).Error("Error")
		}
	})
}
