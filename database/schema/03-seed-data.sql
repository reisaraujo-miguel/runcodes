SET ROLE TO runcodes;

-- Default Languages
-- NOTE: allowed_file_types.extension is UNIQUE, so each extension appears
-- once. Zip archives are represented by the Makefile entry: a .zip submission
-- is compiled with make when it contains a Makefile, otherwise the judge
-- detects the language from the archive contents.
INSERT INTO allowed_file_types (name, extension, is_compilable, is_available)
VALUES
  ('Python 3', 'py', TRUE, TRUE),
  ('C', 'c', TRUE, TRUE),
  ('C++', 'cpp', TRUE, TRUE),
  ('Haskell', 'hs', TRUE, TRUE),
  ('Makefile', 'zip', TRUE, TRUE),
  ('Fortran', 'f', TRUE, TRUE),
  ('Java 17', 'java', TRUE, TRUE),
  ('Pascal', 'pas', TRUE, TRUE),
  ('Portugol 2.6', 'por', TRUE, TRUE),
  ('R', 'r', TRUE, TRUE),
  ('Rust', 'rs', TRUE, TRUE),
  ('PDF', 'pdf', FALSE, TRUE),
  ('Golang', 'go', TRUE, TRUE),
  ('Octave', 'm', TRUE, TRUE),
  ('C#', 'cs', TRUE, TRUE),
  ('Lua', 'lua', TRUE, TRUE),
  ('Prolog', 'pl', TRUE, TRUE),
  ('C (OpenMP)', 'omp.c', TRUE, TRUE),
  ('C++ (OpenMP)', 'omp.cpp', TRUE, TRUE),
  ('C (OpenMP + MPI)', 'mpi.c', TRUE, TRUE),
  ('C++ (OpenMP + MPI)', 'mpi.cpp', TRUE, TRUE),
  ('Verilog', 'v', TRUE, TRUE),
  ('Zig', 'zig', TRUE, TRUE);


-- Platform settings. These are the defaults the admin panel starts from; the
-- frontend build-time VITE_CONTACT_* values are only a fallback for a client
-- that cannot reach the API.
INSERT INTO platform_settings (key, value)
VALUES
  ('contact_email', 'contact@example.com'),
  ('contact_disclaimer_html', 'Em caso de eventuais problemas com a plataforma, entre em contato com <a href="mailto:contact@example.com">contact@example.com</a>');

-- The platform needs an owner, and the seed creates the account — but not a
-- password. A hash committed to this repository is a published credential: this
-- file used to ship `admin@admin.com` with a fixed bcrypt hash and its plaintext
-- in a comment above it.
--
-- `password_hash` is NOT NULL, so the row carries a value that is not a hash and
-- therefore cannot match any password (`verifyBcryptPassword` answers such a
-- login with 401, not 500).
--
-- Set a real password before the first login:
--
--   RUNCODES_ADMIN_PASSWORD='...' ./database/bootstrap-admin.sh
INSERT INTO users (name, email, password_hash, role, confirmed)
VALUES ('admin', 'admin@admin.com', '!', 'admin', TRUE);
