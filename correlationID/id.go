package correlationID

import (
	"context"
	"net/http"

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
	req = req.WithContext(ctx)

	return req, corrID
}

func FromContext(ctx context.Context) string {
	val, ok := ctx.Value(correlationID).(string)
	if ok {
		return val
	}
	return ""
}
