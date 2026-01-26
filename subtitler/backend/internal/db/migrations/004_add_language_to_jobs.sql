-- Migration 004: Add language column to jobs table
-- This allows users to specify the language for transcription (ISO 639-1 code)
-- NULL value means auto-detect

ALTER TABLE jobs ADD COLUMN language TEXT;
