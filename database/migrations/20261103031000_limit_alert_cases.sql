-- A limit alert says which limit is failing and how many submissions it caught;
-- naming the test cases the failures concentrate in is what tells the professor
-- whether one expensive case or the whole exercise needs a looser limit. The
-- email already carries them, so the record keeps them too and the in-app alert
-- can show the same numbers without re-aggregating a window that may still be
-- open. An alert with no case detail (nothing to attribute) stores an empty
-- array rather than NULL, so readers never have to special-case it.
ALTER TABLE "exercise_limit_alerts"
  ADD COLUMN "cases" jsonb DEFAULT '[]'::jsonb NOT NULL;
