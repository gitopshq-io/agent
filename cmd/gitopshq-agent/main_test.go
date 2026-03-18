package main

import (
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestShouldBootstrapWithRegistrationToken(t *testing.T) {
	unauthenticated := fmt.Errorf("connect failed: %w", status.Error(codes.Unauthenticated, "invalid agent token"))
	if !shouldBootstrapWithRegistrationToken(unauthenticated) {
		t.Fatal("expected wrapped unauthenticated error to trigger bootstrap")
	}

	internal := fmt.Errorf("connect failed: %w", status.Error(codes.Internal, "boom"))
	if shouldBootstrapWithRegistrationToken(internal) {
		t.Fatal("did not expect internal error to trigger bootstrap")
	}
}
