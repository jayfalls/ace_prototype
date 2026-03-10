# Security

## Authentication
- JWT-based stateless authentication
- Token includes: user_id, email, exp (1 hour expiry)
- Refresh token before expiration

## Password Security
- bcrypt hashing with cost factor 12
- No plaintext password storage

## API Security
- All routes require JWT except:
  - POST /api/v1/auth/register
  - POST /api/v1/auth/login
- Bearer token in Authorization header

## Data Protection
- SQL injection: Using parameterized queries (simulated with in-memory)
- XSS: Svelte auto-escapes output
- API keys: Stored encrypted (simulated)

## WebSocket Security
- Validate JWT on connection
- Close on heartbeat failure

## Input Validation
- Email format validation
- Password minimum 8 characters
- Input sanitization on all endpoints
