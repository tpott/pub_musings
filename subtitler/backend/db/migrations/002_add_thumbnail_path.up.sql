-- Add thumbnail_path column to videos table
-- Stores path to encrypted thumbnail image (.age extension)

ALTER TABLE videos ADD COLUMN thumbnail_path TEXT;
