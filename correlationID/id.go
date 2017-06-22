package correlationID

import (
	"context"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/google/uuid"
)

const (
	CORRID string = "X-Correlation-Id"
)

var (
	correlationID key
)

type key struct{}

func FromRequest(req *http.Request) (*http.Request, string) {
	corrID := req.Header.Get(CORRID)
	if len(corrID) == 0 {
		corrID = uuid.New().String()
	}
	ctx := context.WithValue(req.Context(), correlationID, corrID)
	log.WithField("x-correlation-id", FromContext(ctx)).Info("FromRequest")
	req = req.WithContext(ctx)
	log.WithField("x-correlation-id", FromContext(req.Context())).Info("FromRequest")

	return req, corrID
}

func FromContext(ctx context.Context) string {
	val, ok := ctx.Value(correlationID).(string)
	if ok {
		return val
	}
	return ""
}
