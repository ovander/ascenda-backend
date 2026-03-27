-- Migration: drop orphaned year_index columns from staff tables
-- Background: GORM originally mapped YearIndex → year_index for the three staff
-- tables, but the DB schema uses "year" as the column name. After adding
-- gorm:"column:year" to the StaffHeadcount/StaffSalary/StaffIncentive models,
-- AutoMigrate left the old year_index columns in place (NOT NULL, no default),
-- causing every INSERT to fail with a not-null constraint violation.
-- This migration removes the orphaned columns.

ALTER TABLE staff_headcounts  DROP COLUMN IF EXISTS year_index;
ALTER TABLE staff_salaries    DROP COLUMN IF EXISTS year_index;
ALTER TABLE staff_incentives  DROP COLUMN IF EXISTS year_index;
