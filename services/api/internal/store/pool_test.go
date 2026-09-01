package store

import (
	"context"
	"testing"
)

func TestOpenRejectsInvalidPoolSettingsBeforeConnecting(t *testing.T) {
	tests := []PoolSettings{
		{MaxConns: 0, MinConns: 0},
		{MaxConns: 1, MinConns: -1},
		{MaxConns: 1, MinConns: 2},
	}
	for _, settings := range tests {
		if _, err := OpenWithPoolSettings(context.Background(), "postgres://unused", settings); err == nil {
			t.Fatalf("accepted settings %#v", settings)
		}
	}
}
