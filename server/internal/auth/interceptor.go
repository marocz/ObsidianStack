package auth

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// APIKeyInterceptor returns a gRPC UnaryServerInterceptor that enforces API key
// authentication on every incoming call.
//
// Behaviour:
//   - If mode != "apikey" or key == "", all calls are allowed (pass-through).
//   - Otherwise the interceptor reads the value of header from the incoming
//     gRPC metadata and compares it to key.
//   - A missing, empty, or incorrect key returns codes.Unauthenticated.
//
// header should be a lowercase string (gRPC metadata keys are case-insensitive
// but are normalised to lowercase by the gRPC library).
func APIKeyInterceptor(mode, header, key string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Non-apikey modes or unconfigured key → allow everything.
		if mode != "apikey" || key == "" {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		vals := md.Get(header)
		if len(vals) == 0 || vals[0] != key {
			return nil, status.Error(codes.Unauthenticated, "invalid api key")
		}

		return handler(ctx, req)
	}
}
