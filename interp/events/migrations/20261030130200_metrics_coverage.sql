-- +goose Up
-- +goose StatementBegin
-- databases created before migrations were tracked may already have the table, ensure it exists so it can be moved.
CREATE TABLE IF NOT EXISTS "eg.metrics.coverage" (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    path TEXT NOT NULL,
    statements FLOAT4 NOT NULL,
    branches FLOAT4 NOT NULL,
    fnname TEXT NOT NULL DEFAULT '',
    hits BIGINT NOT NULL DEFAULT 0
);
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE "eg.metrics.coverage" RENAME TO "eg.metrics.coverage_old";
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE "eg.metrics.coverage" (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    path TEXT NOT NULL,
    statements FLOAT4 NOT NULL,
    branches FLOAT4 NOT NULL,
    fnname TEXT NOT NULL DEFAULT '',
    hits BIGINT NOT NULL DEFAULT 0,
    generated BOOLEAN NOT NULL DEFAULT false
);
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO "eg.metrics.coverage" (id, path, statements, branches, fnname, hits)
SELECT id, path, statements, branches, fnname, hits FROM "eg.metrics.coverage_old";
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE "eg.metrics.coverage_old";
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS "eg.metrics.coverage";
-- +goose StatementEnd
