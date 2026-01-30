-- Migration 006: Add embedded_subtitles_json column to videos table
-- Stores detected embedded subtitle tracks as JSON for display and extraction

ALTER TABLE videos ADD COLUMN embedded_subtitles_json TEXT DEFAULT NULL;
