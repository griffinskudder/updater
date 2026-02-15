-- Change releases FK from CASCADE to RESTRICT
-- This prevents deleting an application that still has releases
ALTER TABLE releases DROP CONSTRAINT releases_application_id_fkey;
ALTER TABLE releases ADD CONSTRAINT releases_application_id_fkey
    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE RESTRICT;
