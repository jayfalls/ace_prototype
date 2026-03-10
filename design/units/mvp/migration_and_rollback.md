# Migration and Rollback

## MVP Data Model

Since MVP uses in-memory storage, no database migration is required.

### Storage Structure
```go
// In-memory stores
agents     map[string]*Agent
sessions   map[string]*Session
memories   map[string]*Memory
thoughts   map[string]*Thought
providers  map[string]*Provider
tools      map[string]*AgentTool
users      map[string]*User
```

### Future PostgreSQL Migration
When migrating to PostgreSQL:
1. Create initial schema with sqlc
2. Write migration scripts in design/units/core-infra/migrations/
3. Add migration runner to application startup
4. Seed with test data

### Rollback Strategy
- MVP: No rollback needed (in-memory)
- Future: Use versioned migration files with down scripts

## Configuration Changes
- All config via environment variables
- No config migration needed for MVP
