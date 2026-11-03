-- The judge grades a submission against the limits snapshot of its commit, so it
-- must be able to read it. The snapshot is per-commit data the judge already
-- needs (like `commits` itself), so it carries no row-level policy, and the
-- judge role gets a plain read grant.
ALTER TABLE "commit_test_case_limits" DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "runcodes_app_only" ON "commit_test_case_limits";

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'runcodes_judge') THEN
        GRANT SELECT ON "commit_test_case_limits" TO runcodes_judge;
    END IF;
END $$;
