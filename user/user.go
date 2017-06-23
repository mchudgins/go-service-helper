package user

import (
	"context"
	"net/http"
)

const (
	USERID string = "X-Remote-User"
)

var (
	userID key
)

type key struct{}

func FromRequest(req *http.Request) (*http.Request, string) {
	id := req.Header.Get(USERID)
	if len(id) > 0 {
		ctx := NewContext(req.Context(), id)
		req = req.WithContext(ctx)
	}

	return req, id
}

func FromContext(ctx context.Context) string {
	val, ok := ctx.Value(userID).(string)
	if ok {
		return val
	}
	return ""
}

func NewContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userID, id)
}
