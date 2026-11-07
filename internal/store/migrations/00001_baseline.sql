-- +goose Up
-- Baseline: creates goose_db_version. Schema arrives in later migrations.
SELECT 1;

-- +goose Down
SELECT 1;
