package model

import "errors"

// Authentication errors
var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUserNotFound        = errors.New("user not found")
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrAccountSuspended    = errors.New("account suspended")
	ErrTokenExpired        = errors.New("token expired")
	ErrTokenRevoked        = errors.New("token revoked")
	ErrRefreshTokenInvalid = errors.New("refresh token invalid")
	ErrWeakPassword        = errors.New("password does not meet requirements")
	ErrRateLimited         = errors.New("rate limit exceeded")
)

// Authorization errors
var (
	ErrResourceAccessDenied = errors.New("resource access denied")
)
