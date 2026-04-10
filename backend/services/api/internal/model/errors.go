package model

import (
	"errors"
)

// Auth errors
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrAccountSuspended   = errors.New("account suspended")
	ErrAccountDeleted     = errors.New("account deleted")
	ErrAccountLocked      = errors.New("account locked due to too many failed attempts")
)

// Token errors
var (
	ErrTokenInvalid        = errors.New("token is invalid")
	ErrTokenExpired        = errors.New("token has expired")
	ErrTokenRevoked        = errors.New("token has been revoked")
	ErrTokenNotFound       = errors.New("token not found")
	ErrTokenAlreadyUsed    = errors.New("token has already been used")
	ErrRefreshTokenInvalid = errors.New("refresh token is invalid")
	ErrRefreshTokenExpired = errors.New("refresh token has expired")
)

// Permission errors
var (
	ErrResourceAccessDenied   = errors.New("resource access denied")
	ErrResourceNotFound       = errors.New("resource not found")
	ErrInsufficientPermission = errors.New("insufficient permission")
)

// Password errors
var (
	ErrWeakPassword           = errors.New("password does not meet complexity requirements")
	ErrPasswordTooShort       = errors.New("password is too short")
	ErrPasswordRequiresUpper  = errors.New("password must contain at least one uppercase letter")
	ErrPasswordRequiresLower  = errors.New("password must contain at least one lowercase letter")
	ErrPasswordRequiresNumber = errors.New("password must contain at least one number")
	ErrPasswordRequiresSymbol = errors.New("password must contain at least one symbol")
	ErrCurrentPasswordInvalid = errors.New("current password is incorrect")
)

// Rate limiting errors
var (
	ErrRateLimited     = errors.New("rate limit exceeded")
	ErrTooManyAttempts = errors.New("too many attempts")
)
