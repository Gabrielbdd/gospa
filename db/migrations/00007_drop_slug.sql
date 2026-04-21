-- Slice 7 — Wave 2 of the slug removal plan.
--
-- Wave 1 (00004 + Slice 2 handler edits) stopped writing meaningful
-- slug values. This migration drops the column entirely so the insert
-- path does not trip the NOT NULL constraint on the workspace-company
-- materialisation inside the install orchestrator.
--
-- Pre-v1: no production data to preserve, no read-repair, no backfill.
-- The column is deleted outright and the partial unique index goes
-- with it.
-- +goose Up
DROP INDEX IF EXISTS companies_slug_active_unique;

ALTER TABLE companies DROP COLUMN IF EXISTS slug;

ALTER TABLE workspace DROP COLUMN IF EXISTS slug;

-- +goose Down
-- Restore the column as NULLABLE. The unique index required every row
-- to have a slug, which meant we would have to invent one for each
-- existing row to reinstate it — pre-v1 there is no value in that.
-- Operators who need slugs back can re-add them explicitly with a
-- follow-up migration.
ALTER TABLE workspace ADD COLUMN IF NOT EXISTS slug TEXT NOT NULL DEFAULT '';

ALTER TABLE companies ADD COLUMN IF NOT EXISTS slug TEXT;
