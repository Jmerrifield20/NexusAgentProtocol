-- Migration 008: user profile fields
ALTER TABLE users
  ADD COLUMN bio            TEXT    NOT NULL DEFAULT '',
  ADD COLUMN avatar_url     TEXT    NOT NULL DEFAULT '',
  ADD COLUMN website_url    TEXT    NOT NULL DEFAULT '',
  ADD COLUMN public_profile BOOLEAN NOT NULL DEFAULT TRUE;
