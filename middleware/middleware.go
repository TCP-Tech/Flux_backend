package middleware

/*
	context key types are used to avoid conflicts when sharing data via contexts
	visit https://vld.bg/articles/go-context-type/ for more info
*/
type contextKey string

const (
	KeyJwtSessionCookieName            = "jwt_session"
	KeyCtxUserCredClaims    contextKey = "UserCredClaims"
)
