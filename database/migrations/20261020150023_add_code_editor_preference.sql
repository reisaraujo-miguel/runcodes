-- Add the in-browser code editor preference to the user profile.

-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "code_editor_enabled" boolean NOT NULL DEFAULT true;
