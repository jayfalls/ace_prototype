package migrations

import (
	"database/sql"

	"github.com/pressly/goose/v3"
)

// upCreateResourcePermissions creates the resource_permissions table.
func upCreateResourcePermissions(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE resource_permissions (
			id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			resource_type  VARCHAR(50) NOT NULL,
			resource_id    UUID NOT NULL,
			permission_level VARCHAR(20) NOT NULL 
			                CHECK (permission_level IN ('view', 'use', 'admin')),
			granted_by     UUID REFERENCES users(id),
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			
			UNIQUE(user_id, resource_type, resource_id)
		);

		CREATE INDEX idx_resource_permissions_user_id ON resource_permissions(user_id);
		CREATE INDEX idx_resource_permissions_resource ON resource_permissions(resource_type, resource_id);

		COMMENT ON TABLE resource_permissions IS 'Resource-level permissions for fine-grained access control.';
		COMMENT ON COLUMN resource_permissions.user_id IS 'User who has the permission.';
		COMMENT ON COLUMN resource_permissions.resource_type IS 'Type of resource: agent, tool, skill, config.';
		COMMENT ON COLUMN resource_permissions.resource_id IS 'ID of the resource.';
		COMMENT ON COLUMN resource_permissions.permission_level IS 'Permission: view, use, or admin.';
		COMMENT ON COLUMN resource_permissions.granted_by IS 'User who granted this permission.';
	`)
	return err
}

// downCreateResourcePermissions drops the resource_permissions table.
func downCreateResourcePermissions(tx *sql.Tx) error {
	_, err := tx.Exec(`
		DROP INDEX IF EXISTS idx_resource_permissions_user_id;
		DROP INDEX IF EXISTS idx_resource_permissions_resource;
		DROP TABLE IF EXISTS resource_permissions;
	`)
	return err
}

func init() {
	goose.AddMigration(upCreateResourcePermissions, downCreateResourcePermissions)
}
