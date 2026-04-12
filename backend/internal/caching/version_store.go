package caching

// =============================================================================
// Version Store — Deferred Integration
// =============================================================================
//
// The versionStore interface and WithVersionStore option are intentionally NOT
// defined here. The infrastructure exists in the database layer:
//
//   - Migration:  backend/services/api/migrations/20260406000000_create_version_stamps.go
//   - SQLC queries: backend/services/api/internal/repository/queries/version_stamps.sql
//   - Generated code: backend/services/api/internal/repository/generated/version_stamps.sql.go
//
// When a service needs version-backed invalidation, it should:
//
//   1. Implement a versionStore adapter that wraps the SQLC-generated queries.
//      The adapter must convert between:
//        - caching.VersionStamp (Key string, Version string, SourceHash string,
//          UpdatedAt time.Time, UpdatedBy string)
//        - db.VersionStamp (Key string, Version string, SourceHash pgtype.Text,
//          UpdatedAt pgtype.Timestamptz, UpdatedBy pgtype.Text)
//
//   2. Add the versionStore interface and WithVersionStore CacheOption to this file.
//
//   3. Add versionStore field to cacheConfig and cacheImpl.
//
//   4. Wire the versionStore into GetOrFetch, Set, and InvalidateByVersion as needed.
//
// This follows the "Always Do Minimal Changes" principle — no dead code until
// there is a consumer.
