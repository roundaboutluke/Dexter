ALTER TABLE humans
  ADD COLUMN community_membership TEXT NOT NULL DEFAULT '[]',
  ADD COLUMN area_restriction TEXT NULL,
  ADD COLUMN notes VARCHAR(255) NOT NULL DEFAULT '';
