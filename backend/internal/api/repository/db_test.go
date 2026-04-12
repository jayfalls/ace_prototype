package repository

import (
	"testing"
)

func TestNewDBWithInvalidDSN(t *testing.T) {
	_, err := NewDB("postgres://testuser:testpass@invalid-host-that-does-not-exist:5432/testdb?sslmode=disable")
	if err == nil {
		t.Error("Expected error for invalid host, got nil")
	}
}

func TestDBCloseNilPool(t *testing.T) {
	db := &DB{Pool: nil}
	// Should not panic
	db.Close()
}
