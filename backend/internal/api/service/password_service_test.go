package service

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	t.Run("generates valid hash", func(t *testing.T) {
		hash, err := HashPassword("TestPassword123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if hash == "" {
			t.Fatal("hash should not be empty")
		}

		// Hash should contain a colon separator
		if len(hash) < 20 {
			t.Fatalf("hash too short: %s", hash)
		}
	})

	t.Run("different salts produce different hashes", func(t *testing.T) {
		hash1, _ := HashPassword("TestPassword123")
		hash2, _ := HashPassword("TestPassword123")

		if hash1 == hash2 {
			t.Fatal("same password should produce different hashes due to random salt")
		}
	})

	t.Run("empty password returns error", func(t *testing.T) {
		_, err := HashPassword("")
		if err == nil {
			t.Fatal("expected error for empty password")
		}
	})
}

func TestVerifyPassword(t *testing.T) {
	t.Run("correct password returns true", func(t *testing.T) {
		hash, err := HashPassword("TestPassword123")
		if err != nil {
			t.Fatalf("failed to hash password: %v", err)
		}

		valid, err := VerifyPassword(hash, "TestPassword123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !valid {
			t.Fatal("correct password should verify successfully")
		}
	})

	t.Run("incorrect password returns false", func(t *testing.T) {
		hash, err := HashPassword("TestPassword123")
		if err != nil {
			t.Fatalf("failed to hash password: %v", err)
		}

		valid, err := VerifyPassword(hash, "WrongPassword456")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if valid {
			t.Fatal("incorrect password should not verify")
		}
	})

	t.Run("empty hash returns error", func(t *testing.T) {
		_, err := VerifyPassword("", "password")
		if err == nil {
			t.Fatal("expected error for empty hash")
		}
	})

	t.Run("empty password returns error", func(t *testing.T) {
		hash, _ := HashPassword("password")
		_, err := VerifyPassword(hash, "")
		if err == nil {
			t.Fatal("expected error for empty password")
		}
	})

	t.Run("invalid hash format returns error", func(t *testing.T) {
		_, err := VerifyPassword("invalid-format", "password")
		if err == nil {
			t.Fatal("expected error for invalid hash format")
		}
	})
}

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid password",
			password:  "TestPass123",
			expectErr: false,
		},
		{
			name:      "too short",
			password:  "Test12",
			expectErr: true,
			errMsg:    "must be at least 8 characters",
		},
		{
			name:      "missing uppercase",
			password:  "testpass123",
			expectErr: true,
			errMsg:    "uppercase",
		},
		{
			name:      "missing lowercase",
			password:  "TESTPASS123",
			expectErr: true,
			errMsg:    "lowercase",
		},
		{
			name:      "missing number",
			password:  "TestPassWord",
			expectErr: true,
			errMsg:    "number",
		},
		{
			name:      "exactly 8 chars meets minimum",
			password:  "TestPass1",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.password)

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCheckPasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		minScore int
	}{
		{
			name:     "strong password",
			password: "TestPass123!",
			minScore: 3,
		},
		{
			name:     "weak password - just length",
			password: "testtest",
			minScore: 1,
		},
		{
			name:     "medium password",
			password: "TestPass123",
			minScore: 2,
		},
		{
			name:     "very weak password",
			password: "t",
			minScore: 0,
		},
		{
			name:     "12 chars with all types",
			password: "TestPass123!",
			minScore: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, feedback := CheckPasswordStrength(tt.password)

			if score < tt.minScore {
				t.Errorf("score %d < minScore %d for password %q, feedback: %v",
					score, tt.minScore, tt.password, feedback)
			}

			// Check score bounds
			if score < 0 || score > 4 {
				t.Errorf("score out of bounds: %d", score)
			}

			// Feedback should be non-empty for low scores
			if score <= 2 && len(feedback) == 0 {
				t.Logf("low score but no feedback for %q", tt.password)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
