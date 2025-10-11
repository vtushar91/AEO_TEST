package pkg

import "context"

// contextKey is a private type to avoid collisions in context values
type contextKey string

const (
	userEmailKey contextKey = "userEmail"
	userIDKey    contextKey = "userID"
)

// ------------------- Email -------------------

func WithEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, userEmailKey, email)
}

func GetEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(userEmailKey).(string)
	return email, ok
}

// ------------------- UserID -------------------

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok
}
