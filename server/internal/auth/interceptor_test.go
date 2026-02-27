package auth

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// passHandler is a grpc.UnaryHandler that returns ("ok", nil).
func passHandler(ctx context.Context, req interface{}) (interface{}, error) {
	return "ok", nil
}

func callWithKey(t *testing.T, interceptor grpc.UnaryServerInterceptor, header, key string) (interface{}, error) {
	t.Helper()
	ctx := context.Background()
	if key != "" {
		md := metadata.Pairs(header, key)
		ctx = metadata.NewIncomingContext(ctx, md)
	}
	return interceptor(ctx, nil, &grpc.UnaryServerInfo{}, passHandler)
}

func TestAPIKeyInterceptor_ModeNone_PassesThrough(t *testing.T) {
	i := APIKeyInterceptor("none", "x-api-key", "secret")
	// No key in context — should still pass because mode != "apikey".
	res, err := i(context.Background(), nil, &grpc.UnaryServerInfo{}, passHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "ok" {
		t.Errorf("result: got %v, want ok", res)
	}
}

func TestAPIKeyInterceptor_EmptyKey_PassesThrough(t *testing.T) {
	// key="" means auth is not configured → allow all.
	i := APIKeyInterceptor("apikey", "x-api-key", "")
	res, err := i(context.Background(), nil, &grpc.UnaryServerInfo{}, passHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "ok" {
		t.Errorf("result: got %v, want ok", res)
	}
}

func TestAPIKeyInterceptor_CorrectKey_Passes(t *testing.T) {
	i := APIKeyInterceptor("apikey", "x-api-key", "supersecret")
	res, err := callWithKey(t, i, "x-api-key", "supersecret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "ok" {
		t.Errorf("result: got %v, want ok", res)
	}
}

func TestAPIKeyInterceptor_WrongKey_Unauthenticated(t *testing.T) {
	i := APIKeyInterceptor("apikey", "x-api-key", "supersecret")
	_, err := callWithKey(t, i, "x-api-key", "wrong")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if code := status.Code(err); code != codes.Unauthenticated {
		t.Errorf("code: got %v, want Unauthenticated", code)
	}
}

func TestAPIKeyInterceptor_MissingHeader_Unauthenticated(t *testing.T) {
	i := APIKeyInterceptor("apikey", "x-api-key", "supersecret")
	// Empty metadata — header absent.
	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
	_, err := i(ctx, nil, &grpc.UnaryServerInfo{}, passHandler)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if code := status.Code(err); code != codes.Unauthenticated {
		t.Errorf("code: got %v, want Unauthenticated", code)
	}
}

func TestAPIKeyInterceptor_NoMetadata_Unauthenticated(t *testing.T) {
	i := APIKeyInterceptor("apikey", "x-api-key", "supersecret")
	// Plain context with no metadata at all.
	_, err := i(context.Background(), nil, &grpc.UnaryServerInfo{}, passHandler)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if code := status.Code(err); code != codes.Unauthenticated {
		t.Errorf("code: got %v, want Unauthenticated", code)
	}
}

func TestAPIKeyInterceptor_CustomHeader(t *testing.T) {
	i := APIKeyInterceptor("apikey", "x-obs-token", "mytoken")
	res, err := callWithKey(t, i, "x-obs-token", "mytoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "ok" {
		t.Errorf("result: got %v, want ok", res)
	}
}
