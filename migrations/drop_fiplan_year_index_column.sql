-- Migration: drop orphaned year_index column from fiplan_entries
-- Background: GORM originally mapped YearIndex → year_index, but the DB schema
-- uses "year" as the column name. After adding gorm:"column:year" to FiplanEntry,
-- AutoMigrate left the old year_index column in place (NOT NULL, no default),
-- causing inserts to fail with "null value in column year_index".
ALTER TABLE fiplan_entries DROP COLUMN IF EXISTS year_index;
