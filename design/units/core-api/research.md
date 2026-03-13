# Research

## Overview
This document evaluates different technical approaches for establishing API & DB patterns. Research is conducted after understanding user requirements (user stories) to ensure we choose the right tools for the job.

## 1. Web Framework

### Options Evaluated

| Framework | Type | Performance | Features | Learning Curve | Maintenance |
|-----------|------|-------------|----------|----------------|-------------|
| **Chi v5** | Router | Very High | Minimal | Low | Active |
| **Gin** | Framework | High | Rich (batteries-included) | Low | Very Active |
| **Fiber** | Framework | Highest | Rich (Express-like) | Low | Very Active |
| **Echo** | Framework | High | Rich | Low | Active |
| **net/http** | Standard Lib | High | None | Very Low | Built-in |

### Analysis
- **Chi**: Minimalist, Go-native, idiomatic. Uses standard `http.Handler` interface. Great for control and minimal dependencies.
- **Gin**: Currently most popular, fastest router category. Batteries-included with middleware, rendering, binding. Large community.
- **Fiber**: Fastest overall (uses fasthttp). Good if performance is critical. Express.js-like API.
- **Echo**: Feature-rich, similar to Gin. Good documentation.

### Recommendation
**Chi** - Selected for:
- Minimal dependencies
- Uses standard Go `net/http` patterns (no framework lock-in)
- Excellent for building your own patterns on top
- Works well with SQLC (pure Go, no ORM needed)
- Go-native feel that scales
- Still very high performance

## 2. Database Access Pattern

### Options Evaluated

| Approach | Type Safety | Flexibility | Complexity | SQL Knowledge |
|----------|-------------|-------------|------------|---------------|
| **SQLC** | Compile-time | High | Low | Required |
| **GORM** | Runtime | Medium | Medium | Not required |
| **sqlx** | None | High | Low | Required |
| **Repository Pattern** | Custom | High | Medium | Required |

### Analysis
- **SQLC**: Generates type-safe Go code from SQL queries. No runtime overhead. Best for type safety.
- **GORM**: Full ORM, auto-migrations, easy to start. Can become complex for advanced queries.
- **sqlx**: Raw SQL with struct mapping. More flexible but no type safety.
- **Repository Pattern**: Interface-based, testable, but requires more boilerplate.

### Recommendation
**SQLC with Repository Pattern** - Selected for:
- Type-safe queries at compile time (catches errors early)
- SQLC is explicitly required by architecture
- Repository pattern provides testability
- Best of both worlds: type-safe queries + testable abstractions

## 3. Migration Tool

### Options Evaluated

| Tool | Language | Rollback | SQL/Go Migrations | Active Maintenance |
|------|----------|-----------|-------------------|---------------------|
| **Goose** | Go | Yes | Both | Active |
| **golang-migrate** | CLI | Yes | Both | Very Active |
| **dbmate** | Ruby/Go | Yes | SQL only | Active |
| **Atlas** | Go | Yes | Both | Very Active |

### Analysis
- **Goose**: Lightweight, SQL-based migrations, Go API available. Simple to use.
- **golang-migrate**: Most popular, pure CLI, many database support.
- **dbmate**: Simple, designed for simplicity, uses plain SQL.
- **Atlas**: Modern, supports schema drift detection, infrastructure-as-code approach.

### Recommendation
**Goose** - Selected for:
- SQL-based migrations (simpler, more portable)
- Go API available (can embed in application)
- Simple versioning
- Works well with SQLC workflow
- User mentioned "Ideally Goose" in problem space

## 4. Input Validation

### Options Evaluated

| Library | Approach | Customization | Error Messages |
|---------|----------|---------------|-----------------|
| **go-playground/validator** | Struct tags | High | Customizable |
| **ozzo-validation** | Code-based | Very High | Customizable |
| **govalidator** | Struct tags | Medium | Customizable |

### Analysis
- **go-playground/validator**: Most popular, struct tags approach, extensive validation rules, easy custom validators.
- **ozzo-validation**: Code-based, more flexible but verbose.
- **govalidator**: Similar to validator but older.

### Recommendation
**go-playground/validator** - Selected for:
- Most widely used and maintained
- Struct tags are clean and declarative
- Works well with Chi/Gin binding
- Good error message customization
- Easy to extend with custom validators

## 5. Project Structure

### Options Evaluated

| Structure | Complexity | Best For | Drawbacks |
|-----------|------------|----------|-----------|
| **Layered (MVC)** | Low | Simple APIs, small teams | Can become anaemic |
| **Domain-Driven (DDD)** | High | Complex domains | Steep learning curve |
| **Clean Architecture** | Medium | Long-term projects | More boilerplate |

### Analysis
- **Layered**: Handlers → Services → Repositories. Simple, familiar, easy to understand.
- **DDD**: Bounded contexts, entities, value objects. For complex business logic.
- **Clean**: Dependencies point inward, business logic in core.

### Recommendation
**Layered (Handler → Service → Repository)** - Selected for:
- Simple and straightforward
- Works well with SQLC (repositories use generated code)
- Easy for AI agents to understand and extend
- Appropriate for API-focused project
- Can evolve to DDD if needed later

## 6. Configuration

### Options Evaluated

| Library | Features | Complexity |
|---------|----------|------------|
| **Standard os.Getenv** | None | Very Low |
| **envconfig** | Env parsing to structs | Low |
| **viper** | Multi-format, remote | High |
| **godotenv** | .env file loading | Low |

### Recommendation
**Standard os.Getenv with godotenv** - Selected for:
- Keep it simple for foundation
- Load .env for development
- Use environment variables in production
- Can add complexity later if needed

## Summary of Recommendations

| Category | Recommendation | Rationale |
|----------|----------------|-----------|
| Web Framework | Chi v5 | Minimal, Go-native, high performance |
| Database | SQLC + Repository | Type-safe, testable |
| Migrations | Goose | SQL-based, simple |
| Validation | go-playground/validator | Popular, struct tags |
| Structure | Layered (Handler/Service/Repository) | Simple, extensible |
| Config | os.Getenv + godotenv | Simple, sufficient |

## Open Questions for Architecture/FSD
- Exact package structure within layered architecture
- Error response format standardization
- Middleware stack composition
- Logging approach (defer to observability unit)

## References
- [Chi v5 GitHub](https://github.com/go-chi/chi)
- [SQLC](https://sqlc.dev/)
- [Goose](https://github.com/pressly/goose)
- [go-playground/validator](https://github.com/go-playground/validator)
- [Go project structure best practices](https://glukhov.org/post/2025/12/go-project-structure/)
