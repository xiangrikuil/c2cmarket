ALTER TABLE IF EXISTS contact_methods
DROP CONSTRAINT IF EXISTS fk_contact_methods_current_version;

DROP TABLE IF EXISTS contact_method_versions;
DROP TABLE IF EXISTS contact_methods;
