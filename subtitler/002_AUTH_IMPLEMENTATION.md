# 002_AUTH_IMPLEMENTATION

## Overview

Implement email+password authentication with user registration and login. This establishes the foundation for user-specific features like job history and personalized dashboards.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Session strategy | JWT tokens | Stateless, simple to implement, scales well |
| Password hashing | bcrypt | Industry standard, built-in to Go |
| Token storage | HTTP-only cookies | Secure against XSS, automatic with requests |
| Token expiry | 7 days | Balance between security and UX |
| Password requirements | Min 8 characters | Simple initial requirement, can enhance later |

## Implementation Plan

### 1. Auth Package (`backend/internal/auth/`)

Create auth utilities:
- `auth.go` - Password hashing/verification, JWT generation/validation
- `auth_test.go` - Unit tests for auth functions
- `middleware.go` - Auth middleware for protected routes
- `middleware_test.go` - Middleware tests

Key functions:
```go
func HashPassword(password string) (string, error)
func VerifyPassword(hash, password string) bool
func GenerateJWT(userID int64, email string) (string, error)
func ValidateJWT(tokenString string) (*Claims, error)
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc
```

### 2. API Endpoints (`backend/cmd/server/`)

Add handlers:
- `POST /api/register` - Create new user account
  - Request: `{"email": "user@example.com", "password": "secret"}`
  - Response: `{"success": true, "user": {"id": 1, "email": "..."}}`
  - Sets JWT cookie

- `POST /api/login` - Authenticate existing user
  - Request: `{"email": "user@example.com", "password": "secret"}`
  - Response: `{"success": true, "user": {"id": 1, "email": "..."}}`
  - Sets JWT cookie

- `POST /api/logout` - Clear session
  - Response: `{"success": true}`
  - Clears JWT cookie

- `GET /api/me` - Get current user (protected)
  - Response: `{"user": {"id": 1, "email": "..."}}`
  - Requires valid JWT

### 3. Database Integration

Users table already exists from Task 5:
```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

Database functions needed:
- `CreateUser(email, passwordHash string) (*User, error)`
- `GetUserByEmail(email string) (*User, error)`
- `GetUserByID(id int64) (*User, error)`

### 4. Environment Configuration

Add to `backend/internal/config/config.go`:
```go
JWTSecret string // Secret key for signing JWTs
```

Load from environment variable `JWT_SECRET` with sensible default for development.

### 5. Testing Strategy

**Unit tests:**
- Password hashing/verification
- JWT generation/validation
- Auth middleware behavior

**Integration tests:**
- Registration flow end-to-end
- Login flow end-to-end
- Protected route access
- Invalid credentials handling
- Duplicate email handling

**Manual verification:**
```bash
# Register
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"testpass123"}' \
  -c cookies.txt

# Login
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"testpass123"}' \
  -c cookies.txt

# Access protected route
curl -X GET http://localhost:8080/api/me \
  -b cookies.txt
```

## Dependencies

Required Go packages:
- `golang.org/x/crypto/bcrypt` - Password hashing
- `github.com/golang-jwt/jwt/v5` - JWT implementation

## Security Considerations

1. **Password security:**
   - Use bcrypt with appropriate cost factor (12-14)
   - Never log passwords
   - Never return password hashes in API responses

2. **JWT security:**
   - Use strong secret key
   - Set appropriate expiration
   - Use HTTP-only cookies to prevent XSS
   - Consider adding refresh token flow later

3. **Input validation:**
   - Validate email format
   - Enforce password requirements
   - Sanitize error messages (don't leak user existence)

4. **Rate limiting:**
   - Not implemented in this task
   - Add in future iteration to prevent brute force

## Future Enhancements

These are out of scope for Task 6:
- Password reset via email
- Email verification
- OAuth integration
- Two-factor authentication
- Rate limiting
- Account lockout after failed attempts
- Refresh token rotation

## Acceptance Criteria

- ✅ Can register new user with email+password
- ✅ Can login with valid credentials
- ✅ Receive JWT token in HTTP-only cookie
- ✅ Can access protected routes with valid token
- ✅ Cannot access protected routes without token
- ✅ Cannot register duplicate email
- ✅ Cannot login with invalid password
- ✅ All unit tests pass
- ✅ Manual curl commands work as documented
