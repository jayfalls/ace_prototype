package service

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"regexp"
	"unicode"

	"golang.org/x/crypto/argon2"
)

// Argon2id configuration parameters - OWASP recommended for production
const (
	Argon2Memory      = 64 * 1024 // 64 MB in KB
	Argon2Iterations  = 3
	Argon2Parallelism = 4
	Argon2SaltLength  = 16
	Argon2KeyLength   = 32
)

// Password validation configuration
const (
	MinPasswordLength = 8
)

// ValidatePasswordError represents password validation error
type ValidatePasswordError struct {
	Message string
}

func (e *ValidatePasswordError) Error() string {
	return e.Message
}

// HashPassword creates an Argon2id hash of the password with a random salt.
// Returns the hash encoded as a base64 string.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password cannot be empty")
	}

	// Generate cryptographically random salt
	salt := make([]byte, Argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.New("generate salt: failed to read random bytes")
	}

	// Hash password using Argon2id
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		Argon2Iterations,
		Argon2Memory,
		Argon2Parallelism,
		Argon2KeyLength,
	)

	// Encode salt + hash as base64 for storage
	// Format: base64(salt) + ":" + base64(hash)
	encoded := base64.StdEncoding.EncodeToString(salt) + ":" + base64.StdEncoding.EncodeToString(hash)

	return encoded, nil
}

// VerifyPassword compares a password against an Argon2id hash using constant-time comparison.
// Returns true if the password matches, false otherwise.
func VerifyPassword(hash, password string) (bool, error) {
	if hash == "" || password == "" {
		return false, errors.New("hash and password are required")
	}

	// Split stored hash into salt and hash components
	parts := splitHash(hash)
	if len(parts) != 2 {
		return false, errors.New("invalid hash format")
	}

	salt, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return false, errors.New("decode salt: invalid base64")
	}

	storedHash, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return false, errors.New("decode hash: invalid base64")
	}

	// Compute hash of provided password with same salt
	computedHash := argon2.IDKey(
		[]byte(password),
		salt,
		Argon2Iterations,
		Argon2Memory,
		Argon2Parallelism,
		Argon2KeyLength,
	)

	// Use constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare(computedHash, storedHash) == 1, nil
}

// ValidatePasswordStrength checks if a password meets the minimum strength requirements.
// Returns an error with helpful feedback if the password is invalid.
func ValidatePasswordStrength(password string) error {
	if len(password) < MinPasswordLength {
		return &ValidatePasswordError{
			Message: "password must be at least 8 characters long",
		}
	}

	var hasUpper, hasLower, hasNumber bool

	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasNumber = true
		}
	}

	if !hasUpper {
		return &ValidatePasswordError{
			Message: "password must contain at least one uppercase letter",
		}
	}

	if !hasLower {
		return &ValidatePasswordError{
			Message: "password must contain at least one lowercase letter",
		}
	}

	if !hasNumber {
		return &ValidatePasswordError{
			Message: "password must contain at least one number",
		}
	}

	return nil
}

// CheckPasswordStrength evaluates password strength and returns a score with feedback.
// Score ranges from 0-4:
//   - 0: Very weak (fails most criteria)
//   - 1: Weak (only meets length requirement)
//   - 2: Fair (meets length + one character type)
//   - 3: Good (meets length + two character types)
//   - 4: Strong (meets all requirements)
func CheckPasswordStrength(password string) (score int, feedback []string) {
	score = 0
	feedback = []string{}

	// Check length
	if len(password) >= 12 {
		score += 1
		feedback = append(feedback, "Good length (12+ characters)")
	} else if len(password) >= MinPasswordLength {
		score += 1
		// Don't add feedback - this is the minimum
	} else {
		feedback = append(feedback, "Password is too short (minimum 8 characters)")
	}

	// Check character types
	var hasUpper, hasLower, hasNumber, hasSpecial bool

	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasNumber = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}

	if hasUpper {
		score += 1
	} else {
		feedback = append(feedback, "Add uppercase letters")
	}

	if hasLower {
		score += 1
	} else {
		feedback = append(feedback, "Add lowercase letters")
	}

	if hasNumber {
		score += 1
	} else {
		feedback = append(feedback, "Add numbers")
	}

	if hasSpecial {
		score += 1
		feedback = append(feedback, "Good use of special characters")
	}

	// Cap score at 4
	if score > 4 {
		score = 4
	}

	return score, feedback
}

// splitHash splits a stored hash into salt and hash components.
// Expected format: base64(salt):base64(hash)
func splitHash(hash string) []string {
	// Match the format: base64(salt):base64(hash)
	// Use simple split since we control the format
	var parts []string
	var current []byte

	for i := 0; i < len(hash); i++ {
		if hash[i] == ':' && len(parts) == 0 {
			parts = append(parts, string(current))
			current = nil
		} else {
			current = append(current, hash[i])
		}
	}

	if len(current) > 0 {
		parts = append(parts, string(current))
	}

	return parts
}

// isValidBase64 checks if a string is valid base64
func isValidBase64(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// PasswordRegex returns a compiled regex for additional client-side validation
// This is read-only validation that checks format only, not strength
var PasswordRegex = regexp.MustCompile(`^[a-zA-Z0-9!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?~]+$`)

// PIN configuration - 4-6 digits
const (
	MinPINLength = 4
	MaxPINLength = 6
)

// ValidatePIN checks if a PIN meets the requirements (4-6 digits).
func ValidatePIN(pin string) error {
	if len(pin) < MinPINLength || len(pin) > MaxPINLength {
		return &ValidatePasswordError{
			Message: "PIN must be 4-6 digits",
		}
	}
	for _, ch := range pin {
		if ch < '0' || ch > '9' {
			return &ValidatePasswordError{
				Message: "PIN must contain only digits",
			}
		}
	}
	return nil
}

// HashPIN creates a secure hash of the PIN.
// Uses Argon2id but with a smaller memory footprint suitable for PINs.
func HashPIN(pin string) (string, error) {
	if err := ValidatePIN(pin); err != nil {
		return "", err
	}

	// Generate cryptographically random salt
	salt := make([]byte, Argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.New("generate salt: failed to read random bytes")
	}

	// Hash PIN using Argon2id - same as password hashing for security
	hash := argon2.IDKey(
		[]byte(pin),
		salt,
		Argon2Iterations,
		Argon2Memory,
		Argon2Parallelism,
		Argon2KeyLength,
	)

	// Encode salt + hash as base64 for storage
	encoded := base64.StdEncoding.EncodeToString(salt) + ":" + base64.StdEncoding.EncodeToString(hash)

	return encoded, nil
}

// VerifyPIN compares a PIN against a stored hash using constant-time comparison.
func VerifyPIN(hash, pin string) (bool, error) {
	if hash == "" || pin == "" {
		return false, errors.New("hash and PIN are required")
	}

	// Validate PIN format
	if err := ValidatePIN(pin); err != nil {
		return false, err
	}

	// Use the same splitHash function as passwords
	parts := splitHash(hash)
	if len(parts) != 2 {
		return false, errors.New("invalid hash format")
	}

	salt, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return false, errors.New("decode salt: invalid base64")
	}

	storedHash, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return false, errors.New("decode hash: invalid base64")
	}

	// Compute hash of provided PIN with same salt
	computedHash := argon2.IDKey(
		[]byte(pin),
		salt,
		Argon2Iterations,
		Argon2Memory,
		Argon2Parallelism,
		Argon2KeyLength,
	)

	// Use constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare(computedHash, storedHash) == 1, nil
}
