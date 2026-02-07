ALTER TABLE humans
  ADD COLUMN schedule_disabled TINYINT(1) NOT NULL DEFAULT 0,
  ADD COLUMN preferred_profile_no INT NOT NULL DEFAULT 1;

UPDATE humans
SET preferred_profile_no = CASE
  WHEN current_profile_no > 0 THEN current_profile_no
  ELSE preferred_profile_no
END;
