-- Remove thumbnail_path column from videos table
-- SQLite doesn't support DROP COLUMN directly in older versions,
-- but SQLite 3.35.0+ (2021-03-12) supports it

ALTER TABLE videos DROP COLUMN thumbnail_path;
