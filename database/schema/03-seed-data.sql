SET ROLE TO runcodes;

-- Default Languages
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
  ('Zip', 'zip', FALSE, TRUE),
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


-- IMPORTANT: Don't forget to change the default user's password :)

-- Passwords: Admin&1234
INSERT INTO users (name, email, password_hash, role, confirmed) VALUES ('admin', 'admin@admin.com', '$2a$12$oEehawURBhhtdOLAE8ARpem031dAxE4fJGIMjXFCK2hxb65kRIPvC', 'admin', TRUE);
