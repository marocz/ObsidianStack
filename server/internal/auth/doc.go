// Package auth provides authentication middleware for obsidianstack-server.
//
// APIKeyInterceptor(mode, header, key) returns a gRPC UnaryServerInterceptor
// that validates the API key from the named gRPC metadata header.
//
// When mode != "apikey" or key == "", all calls pass through (useful for local
// development with auth disabled). When the key is incorrect or absent,
// the interceptor returns codes.Unauthenticated immediately.
//
// HTTP middleware for the REST API will be added in T012 (Phase 3).
package auth
