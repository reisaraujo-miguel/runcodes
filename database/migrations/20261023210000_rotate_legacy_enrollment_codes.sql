-- Legacy classes were issued four-character enrollment codes. Six characters
-- are the current minimum (36^6 instead of 36^4 combinations), so rotate every
-- surviving short code to a fresh random value before enforcing the floor.
--
-- This is a data migration: codes distributed to students stop working. The
-- class owner can see the new code on the class page and can rotate it again
-- there if needed.
DO $$
DECLARE
	offering record;
	candidate text;
BEGIN
	FOR offering IN
		SELECT id
		FROM offerings
		WHERE enrollment_code IS NOT NULL
		  AND length(enrollment_code) < 6
		FOR UPDATE
	LOOP
		LOOP
			candidate := upper(substr(replace(gen_random_uuid()::text, '-', ''), 1, 6));
			EXIT WHEN NOT EXISTS (
				SELECT 1 FROM offerings WHERE enrollment_code = candidate
			);
		END LOOP;

		UPDATE offerings
		SET enrollment_code = candidate,
		    updated_at = now()
		WHERE id = offering.id;
	END LOOP;
END $$;

-- Make the floor structural. `NULL` is allowed for classes migrated from the
-- legacy system without a code; an owner can generate one through the API.
ALTER TABLE offerings
	ADD CONSTRAINT offerings_enrollment_code_length
	CHECK (enrollment_code IS NULL OR length(enrollment_code) >= 6);
