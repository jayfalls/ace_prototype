# API Error Codes

**FSD Requirement**: FR-2.4

---

## Overview

This document catalogs all API error codes, their HTTP status mappings, database-to-API error translation, and validation error format.

All API responses use the standard envelope defined in `response.go`:

```json
// Success
{ "success": true, "data": {...} }

// Error
{ "success": false, "error": { "code": "...", "message": "...", "details": [...] } }
```

---

## Error Code Catalog

| HTTP Status | Error Code | When It Occurs | Client Handling |
|-------------|------------|----------------|-----------------|
| 400 | `invalid_request` | JSON decode failure | Fix request body format |
| 400 | `validation_error` | Struct validation fails | Check `details` array for field-level errors |
| 400 | `bad_request` | General bad request | Read `message` for details |
| 401 | `unauthorized` | Missing or invalid JWT token | Re-authenticate |
| 403 | `forbidden` | Valid token, insufficient permissions | Request elevated permissions |
| 404 | `not_found` | Resource does not exist | Verify resource ID |
| 409 | `conflict` | Unique constraint violation | Check for duplicates |
| 500 | `internal_error` | Unhandled server error | Retry with backoff, report if persistent |

---

## Response Helper Functions

From `backend/services/api/internal/response/response.go`:

| Function | HTTP Status | Error Code | Use Case |
|----------|-------------|------------|----------|
| `response.Success(w, data)` | 200 | — | Successful read |
| `response.Created(w, data)` | 201 | — | Successful create |
| `response.BadRequest(w, code, msg)` | 400 | caller-provided | Invalid request |
| `response.ValidationError(w, err)` | 400 | `validation_error` | Struct validation failure |
| `response.Unauthorized(w, msg)` | 401 | `unauthorized` | Missing/invalid auth |
| `response.Forbidden(w, msg)` | 403 | `forbidden` | Insufficient permissions |
| `response.NotFound(w, msg)` | 404 | `not_found` | Resource not found |
| `response.InternalError(w, msg)` | 500 | `internal_error` | Server error |
| `response.JSON(w, status, data)` | caller-provided | — | Raw JSON response |

---

## Database-to-API Error Mapping

PostgreSQL errors are mapped to API error codes in the service/handler layer:

| PostgreSQL Error | SQLState | API Status | API Error Code |
|-----------------|----------|------------|----------------|
| Unique violation | `23505` | 409 | `conflict` |
| Foreign key violation | `23503` | 404 | `not_found` |
| Not null violation | `23502` | 400 | `bad_request` |
| Check constraint violation | `23514` | 400 | `bad_request` |
| Connection refused | `08001` | 503 | `service_unavailable` |
| Query timeout | `57014` | 504 | `timeout` |
| Deadlock detected | `40P01` | 409 | `conflict` (retryable) |
| Serialization failure | `40001` | 409 | `conflict` (retryable) |

### Handling Example

```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) {
    switch pgErr.Code {
    case "23505":
        response.Error(w, "conflict", "Resource already exists", http.StatusConflict)
        return
    case "23503":
        response.NotFound(w, "Referenced resource not found")
        return
    case "23502":
        response.BadRequest(w, "bad_request", "Required field is missing")
        return
    default:
        response.InternalError(w, "Database error")
        return
    }
}
```

---

## Validation Error Format

The `response.ValidationError` function produces field-level error details from `go-playground/validator`:

```json
{
  "success": false,
  "error": {
    "code": "validation_error",
    "message": "Invalid request data",
    "details": [
      { "field": "Name", "message": "This field is required" },
      { "field": "Email", "message": "Invalid email format" }
    ]
  }
}
```

### Supported Validation Tags

| Tag | Message |
|-----|---------|
| `required` | "This field is required" |
| `email` | "Invalid email format" |
| `min` | "Value is too short" |
| `max` | "Value is too long" |
| `url` | "Invalid URL format" |
| (default) | "Invalid value" |

### FieldError Structure

| Field | Type | Description |
|-------|------|-------------|
| `field` | string | Name of the field that failed validation |
| `message` | string | Human-readable validation message |

---

## Error Response Examples

### 400 Validation Error

```json
{
  "success": false,
  "error": {
    "code": "validation_error",
    "message": "Invalid request data",
    "details": [
      { "field": "name", "message": "This field is required" },
      { "field": "email", "message": "Invalid email format" }
    ]
  }
}
```

### 401 Unauthorized

```json
{
  "success": false,
  "error": {
    "code": "unauthorized",
    "message": "Missing or invalid token"
  }
}
```

### 404 Not Found

```json
{
  "success": false,
  "error": {
    "code": "not_found",
    "message": "Agent not found"
  }
}
```

### 409 Conflict

```json
{
  "success": false,
  "error": {
    "code": "conflict",
    "message": "Agent with this name already exists"
  }
}
```

### 500 Internal Error

```json
{
  "success": false,
  "error": {
    "code": "internal_error",
    "message": "An unexpected error occurred"
  }
}
```

---

## Notes

- All error codes use `snake_case` convention
- The `code` field is machine-readable — clients should switch on code, not message
- The `message` field is human-readable — suitable for display to end users
- The `details` field is only present for `validation_error` responses
- Database errors are translated at the service/handler layer — SQLState codes are never exposed to clients
