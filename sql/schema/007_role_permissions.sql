CREATE TABLE role_permissions (
  role_id UUID NOT NULL,
  permission_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  PRIMARY KEY (role_id, permission_id),

  CONSTRAINT role_permissions_role_fk
    FOREIGN KEY (role_id)
    REFERENCES roles(id)
    ON DELETE CASCADE,

  CONSTRAINT role_permissions_permission_fk
    FOREIGN KEY (permission_id)
    REFERENCES permissions(id)
    ON DELETE CASCADE
);

CREATE INDEX idx_role_permissions_permission_id
ON role_permissions(permission_id);
