-- Phase 8 (IMPL-0001) placeholder migration. Creates a tiny meta
-- table the migrator can use as a smoke target. Real DDL for
-- watches, listings, components, jobs, tasks, alerts, etc., lands
-- with the datastore IMPL.

-- +goose Up
CREATE TABLE _spt_meta (
    key         text PRIMARY KEY,
    value       text NOT NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

INSERT INTO _spt_meta (key, value)
VALUES ('schema_version', '0.0.1');

-- +goose Down
DROP TABLE _spt_meta;
