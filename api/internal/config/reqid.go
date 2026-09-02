package config

import "context"

type contextKey struct{}

var key = contextKey{}

//Function returns a new context carrying the given REQ ID
func WithID(ctx context.Context, id string) context.Context{
	return context.WithValue(ctx, key, id)
}

func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(key).(string)
	return id 
}

