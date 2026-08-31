-- run.codes — base database schema.
-- Condensed rewrite of the PostgreSQL dump: produces the same schema as
-- base.sql (same tables, columns, constraints, indexes, sequences, ownership
-- and privileges), with one statement per object and no pg_dump boilerplate.

SET statement_timeout = 0; SET lock_timeout = 0; SET client_encoding TO utf8; SET standard_conforming_strings = 'on'; SET check_function_bodies = 'false'; SET client_min_messages = warning;

CREATE USER runcodes;
CREATE DATABASE "runcodes";
\c runcodes
GRANT ALL ON DATABASE runcodes TO runcodes;
REVOKE ALL ON SCHEMA public FROM public, postgres;
GRANT ALL ON SCHEMA public TO postgres, public;

-- All objects below are created by the runcodes role (equivalent to the
-- per-object ALTER ... OWNER TO runcodes statements of the original dump).
SET ROLE TO runcodes;

-- Sequences
CREATE SEQUENCE alerts_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE allowed_files_id_seq START WITH 100 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE allowed_files_exercises_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE blacklist_mails_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE commits_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE commits_exercise_cases_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE compilation_files_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE courses_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE cria_schools_id_seq START WITH 100 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE cria_students_id_seq START WITH 100 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE disk_reports_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE droplets_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE enrollments_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE exercise_case_files_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE exercise_cases_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE exercise_files_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE exercises_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE logs_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE mail_logs_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE messages_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE offerings_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE public_exercises_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE questions_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE tickets_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;
CREATE SEQUENCE universities_id_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;

-- Tables (ordered so that inlined foreign keys can be resolved)
CREATE TABLE courses (
  id int DEFAULT nextval(CAST('courses_id_seq' AS regclass)) NOT NULL,
  code varchar(16) NOT NULL,
  title varchar(255) NOT NULL,
  university_id int DEFAULT 1 NOT NULL,
  CONSTRAINT "pk_courses" PRIMARY KEY (id)
);
CREATE TABLE offerings (
  id int DEFAULT nextval(CAST('offerings_id_seq' AS regclass)) NOT NULL,
  course_id int NOT NULL,
  year int NOT NULL,
  term int NOT NULL,
  classroom varchar(45),
  end_date date NOT NULL,
  visible_to_enroll boolean DEFAULT TRUE,
  enrollment_code varchar(10),
  max_students int DEFAULT 0,
  max_exercises int DEFAULT 0,
  CONSTRAINT "pk_offerings" PRIMARY KEY (id),
  CONSTRAINT "un_offerings_offerings" UNIQUE (course_id, year, term, classroom),
  CONSTRAINT "fk_offerings_courses_code" FOREIGN KEY (course_id) REFERENCES courses (id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE TABLE exercises (
  id int DEFAULT nextval(CAST('exercises_id_seq' AS regclass)) NOT NULL,
  offering_id int NOT NULL,
  title varchar(255) NOT NULL,
  description text,
  deadline timestamp DEFAULT NOW() NOT NULL,
  user_email varchar(255) NOT NULL,
  type int DEFAULT 0 NOT NULL,
  open_date timestamp DEFAULT NOW(),
  show_before_opening boolean,
  cases_change timestamp,
  removed boolean DEFAULT FALSE,
  ghost boolean DEFAULT FALSE,
  real_id int,
  public boolean DEFAULT FALSE,
  markdown boolean DEFAULT FALSE,
  CONSTRAINT "pk_exercises" PRIMARY KEY (id),
  CONSTRAINT "fk_exercises_offerings_id" FOREIGN KEY (offering_id) REFERENCES offerings (id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE TABLE universities (
  id int DEFAULT nextval(CAST('universities_id_seq' AS regclass)) NOT NULL,
  abbreviation varchar(20) NOT NULL,
  name varchar(210) NOT NULL,
  student_identifier_text varchar(40),
  type int DEFAULT 1,
  state varchar(5) DEFAULT CAST('BR' AS varchar),
  CONSTRAINT "universities_pkey" PRIMARY KEY (id)
);
CREATE TABLE users (
  email varchar(255) NOT NULL,
  name varchar(255) NOT NULL,
  password varchar(40) NOT NULL,
  type int NOT NULL,
  creation timestamp DEFAULT NOW() NOT NULL,
  confirmed boolean DEFAULT FALSE,
  university_id int,
  identifier varchar(50),
  source int DEFAULT 0,
  CONSTRAINT "pk_users" PRIMARY KEY (email),
  CONSTRAINT "ck_users_type_user" CHECK (type = ANY (ARRAY[0, 1, 2, 3, 4]))
);
CREATE TABLE alerts (
  id int DEFAULT nextval(CAST('alerts_id_seq' AS regclass)) NOT NULL,
  type int NOT NULL,
  offering_id int,
  recipients int NOT NULL,
  user_email varchar(255) NOT NULL,
  valid date NOT NULL,
  title varchar(100) NOT NULL,
  message varchar(255) NOT NULL,
  CONSTRAINT "pk_alerts" PRIMARY KEY (id),
  CONSTRAINT "ck_alerts_recipient" CHECK (recipients = ANY (ARRAY[0, 1, 2])),
  CONSTRAINT "ck_alerts_type" CHECK (type = ANY (ARRAY[0, 1, 2, 3])),
  CONSTRAINT "fk_alerts_offerings_id" FOREIGN KEY (offering_id) REFERENCES offerings (id) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT "fk_alerts_users_email" FOREIGN KEY (user_email) REFERENCES users (email) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE TABLE allowed_files (
  id int DEFAULT nextval(CAST('allowed_files_id_seq' AS regclass)) NOT NULL,
  name varchar(200) NOT NULL,
  extension varchar(20) NOT NULL,
  compilable boolean,
  compile_command varchar(255),
  run_command varchar(255),
  available boolean DEFAULT TRUE,
  CONSTRAINT "pk_files" PRIMARY KEY (id)
);
CREATE TABLE allowed_files_exercises (
  id int DEFAULT nextval(CAST('allowed_files_exercises_id_seq' AS regclass)) NOT NULL,
  exercise_id int NOT NULL,
  allowed_file_id int,
  CONSTRAINT "pk_exercises_allowed_files" PRIMARY KEY (id),
  CONSTRAINT "fk_allowed_files_exercises_id" FOREIGN KEY (allowed_file_id) REFERENCES allowed_files (id) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT "fk_exercises_allowed_files_id" FOREIGN KEY (exercise_id) REFERENCES exercises (id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE TABLE blacklist_mails (
  id int DEFAULT nextval(CAST('blacklist_mails_id_seq' AS regclass)) NOT NULL,
  type smallint,
  address varchar(200),
  CONSTRAINT "blacklist_mails_pkey" PRIMARY KEY (id)
);
CREATE TABLE commits (
  id int DEFAULT nextval(CAST('commits_id_seq' AS regclass)) NOT NULL,
  user_email varchar(255) NOT NULL,
  exercise_id int NOT NULL,
  commit_time timestamp DEFAULT NOW() NOT NULL,
  status int NOT NULL,
  hash varchar(150),
  corrects int DEFAULT 0,
  score numeric(10, 2) DEFAULT 0.0,
  compiled boolean,
  compiled_message text,
  compilation_started timestamp,
  compilation_finished timestamp,
  compiled_signal varchar(50) DEFAULT CAST('' AS varchar),
  compiled_error text DEFAULT CAST('' AS text),
  ip varchar(30) DEFAULT CAST(NULL AS varchar),
  aws_key varchar(150) DEFAULT CAST(NULL AS varchar),
  CONSTRAINT "pk_commits" PRIMARY KEY (id),
  CONSTRAINT "fk_commits_exercises_id" FOREIGN KEY (exercise_id) REFERENCES exercises (id) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT "fk_commits_users_email" FOREIGN KEY (user_email) REFERENCES users (email) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE TABLE exercise_cases (
  id int DEFAULT nextval(CAST('exercise_cases_id_seq' AS regclass)) NOT NULL,
  exercise_id int NOT NULL,
  input text,
  input_type int NOT NULL,
  output text,
  output_type int NOT NULL,
  show_input boolean,
  show_expected_output boolean,
  maxmemsize int NOT NULL,
  cputime int NOT NULL,
  stacksize int NOT NULL,
  show_user_output boolean,
  file_size int NOT NULL,
  abs_error double precision,
  last_update timestamp DEFAULT NOW(),
  input_md5 varchar(40) DEFAULT CAST(NULL AS varchar),
  output_md5 varchar(40) DEFAULT CAST(NULL AS varchar),
  CONSTRAINT "pk_exercise_cases" PRIMARY KEY (id),
  CONSTRAINT "fk_exercise_cases_exercise_id" FOREIGN KEY (exercise_id) REFERENCES exercises (id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE TABLE commits_exercise_cases (
  id int DEFAULT nextval(CAST('commits_exercise_cases_id_seq' AS regclass)) NOT NULL,
  commit_id int NOT NULL,
  exercise_case_id int NOT NULL,
  cputime numeric(15, 4) NOT NULL,
  memused int NOT NULL,
  output text NOT NULL,
  output_type int NOT NULL,
  status int NOT NULL,
  status_message text,
  error text DEFAULT CAST('' AS text),
  CONSTRAINT "pk_commits_exercise_case" PRIMARY KEY (id),
  CONSTRAINT "fk_commits_exercise_cases_commits_id" FOREIGN KEY (commit_id) REFERENCES commits (id) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT "fk_commits_exercise_cases_exercise_cases_id" FOREIGN KEY (exercise_case_id) REFERENCES exercise_cases (id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE TABLE compilation_files (
  id int DEFAULT nextval(CAST('compilation_files_id_seq' AS regclass)) NOT NULL,
  exercise_id int NOT NULL,
  path varchar(150) NOT NULL,
  CONSTRAINT "compilation_files_pkey" PRIMARY KEY (id),
  CONSTRAINT "fk_compilation_files_exercise" FOREIGN KEY (exercise_id) REFERENCES exercises (id) ON DELETE CASCADE
);
CREATE TABLE cria_schools (
  id int DEFAULT nextval(CAST('cria_schools_id_seq' AS regclass)) NOT NULL,
  name varchar(300) NOT NULL,
  supervisor varchar(500) NOT NULL,
  city varchar(100) NOT NULL,
  state varchar(100) NOT NULL,
  edition int NOT NULL,
  password varchar(10) NOT NULL,
  email varchar(500) NOT NULL,
  next_student_number int DEFAULT 100,
  removed boolean DEFAULT FALSE,
  CONSTRAINT "cria_schools_pkey" PRIMARY KEY (id)
);
CREATE TABLE cria_students (
  id int DEFAULT nextval(CAST('cria_students_id_seq' AS regclass)) NOT NULL,
  name varchar(100) NOT NULL,
  cria_school_id int NOT NULL,
  edition int NOT NULL,
  password varchar(10) NOT NULL,
  number int DEFAULT 999,
  email varchar(150) NOT NULL,
  removed boolean DEFAULT FALSE,
  CONSTRAINT "cria_students_pkey" PRIMARY KEY (id)
);
CREATE TABLE disk_reports (
  id int DEFAULT nextval(CAST('disk_reports_id_seq' AS regclass)) NOT NULL,
  datetime TIMESTAMP WITH TIME ZONE,
  disk varchar(100),
  used real,
  free real,
  size real,
  CONSTRAINT "disk_reports_pkey" PRIMARY KEY (id)
);
CREATE TABLE droplets (
  id int DEFAULT nextval(CAST('droplets_id_seq' AS regclass)) NOT NULL,
  ip inet,
  active boolean NOT NULL,
  memsize int NOT NULL,
  hdsize int NOT NULL,
  place varchar(100) NOT NULL,
  os varchar(100) NOT NULL,
  CONSTRAINT "pk_droplets" PRIMARY KEY (id)
);
ALTER SEQUENCE droplets_id_seq OWNED BY droplets.id;
CREATE TABLE enrollments (
  id int DEFAULT nextval(CAST('enrollments_id_seq' AS regclass)) NOT NULL,
  offering_id int NOT NULL,
  user_email varchar(255) NOT NULL,
  role int NOT NULL,
  banned boolean DEFAULT FALSE,
  CONSTRAINT "pk_enrollments" PRIMARY KEY (id),
  CONSTRAINT "unique_enrollments" UNIQUE (offering_id, user_email),
  CONSTRAINT "ck_enrollments_role_type_user" CHECK (role = ANY (ARRAY[0, 1, 2])),
  CONSTRAINT "fk_enrollments_offerings_id" FOREIGN KEY (offering_id) REFERENCES offerings (id) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT "fk_enrollments_users_email" FOREIGN KEY (user_email) REFERENCES users (email) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE TABLE exercise_case_files (
  id int DEFAULT nextval(CAST('exercise_case_files_id_seq' AS regclass)) NOT NULL,
  exercise_case_id int NOT NULL,
  path varchar(150) NOT NULL,
  created_at timestamp DEFAULT NOW(),
  CONSTRAINT "pk_exercise_case_files" PRIMARY KEY (id),
  CONSTRAINT "fk_exercise_case_files_exercise" FOREIGN KEY (exercise_case_id) REFERENCES exercise_cases (id) ON DELETE CASCADE,
  CONSTRAINT "fk_exercise_case_files_exercise_id" FOREIGN KEY (exercise_case_id) REFERENCES exercise_cases (id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE TABLE exercise_files (
  id int DEFAULT nextval(CAST('exercise_files_id_seq' AS regclass)) NOT NULL,
  exercise_id int NOT NULL,
  path varchar(150) NOT NULL,
  CONSTRAINT "pk_exercise_files" PRIMARY KEY (id),
  CONSTRAINT "fk_exercise_files_exercise_id" FOREIGN KEY (exercise_id) REFERENCES exercises (id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE TABLE ganglia (
  ip varchar(15),
  name varchar(255),
  reported numeric(100, 0),
  boottime numeric(100, 0),
  machinetype varchar(255),
  osname varchar(255),
  osrelease varchar(255),
  proctotal int,
  cpunum int,
  cpuspeed double precision,
  cpuidle double precision,
  ioreads double precision,
  iowrites double precision,
  ionread double precision,
  ionwrite double precision,
  memtotal double precision,
  memcached double precision,
  membuffers double precision,
  memfree double precision,
  swaptotal double precision,
  swapfree double precision,
  disktotal double precision,
  diskfree double precision,
  loadone double precision,
  loadfive double precision,
  loadfifteen double precision,
  pktsin double precision,
  pktsout double precision,
  bytesin double precision,
  bytesout double precision,
  isjail boolean DEFAULT FALSE
);
CREATE TABLE jail_status (
  jailname varchar(255) NOT NULL,
  status int,
  tsmp timestamp,
  ipaddress varchar(39),
  ssh_port int,
  ssh_status int,
  CONSTRAINT "jail_status_pkey" PRIMARY KEY (jailname)
);
CREATE TABLE logs (
  id int DEFAULT nextval(CAST('logs_id_seq' AS regclass)) NOT NULL,
  user_email varchar(255) NOT NULL,
  datetime timestamp DEFAULT NOW() NOT NULL,
  ip varchar(15) NOT NULL,
  action text NOT NULL,
  CONSTRAINT "pk_actionlogs" PRIMARY KEY (id)
);
CREATE TABLE mail_logs (
  id int DEFAULT nextval(CAST('mail_logs_id_seq' AS regclass)) NOT NULL,
  sent_to varchar(150),
  subject varchar(200),
  message text,
  hash varchar(80),
  opened int DEFAULT 0,
  sent_date timestamp DEFAULT NOW(),
  first_opened_time timestamp,
  CONSTRAINT "mail_logs_pkey" PRIMARY KEY (id)
);
CREATE TABLE messages (
  id int DEFAULT nextval(CAST('messages_id_seq' AS regclass)) NOT NULL,
  recipes text,
  subject text,
  message text,
  attachments text,
  template text,
  CONSTRAINT "messages_pkey" PRIMARY KEY (id)
);
CREATE TABLE public_exercises (
  id int DEFAULT nextval(CAST('public_exercises_id_seq' AS regclass)) NOT NULL,
  exercise_id int NOT NULL,
  level int,
  obs text,
  keywords varchar(255),
  CONSTRAINT "public_exercises_pkey" PRIMARY KEY (id)
);
CREATE TABLE questions (
  id int DEFAULT nextval(CAST('questions_id_seq' AS regclass)) NOT NULL,
  title varchar(350),
  text text,
  tags varchar(350),
  CONSTRAINT "questions_pkey" PRIMARY KEY (id)
);
CREATE TABLE tickets (
  id int DEFAULT nextval(CAST('tickets_id_seq' AS regclass)) NOT NULL,
  users_email varchar(255) NOT NULL,
  datetime timestamp DEFAULT NOW() NOT NULL,
  type int NOT NULL,
  status int NOT NULL,
  solved boolean,
  priority int NOT NULL,
  message text NOT NULL,
  CONSTRAINT "pk_tickets" PRIMARY KEY (id),
  CONSTRAINT "fk_tickets_users_email" FOREIGN KEY (users_email) REFERENCES users (email) ON DELETE CASCADE ON UPDATE CASCADE
);

-- Indexes
CREATE INDEX "courses_university_id_idx" ON courses USING btree (university_id);
CREATE INDEX "idx_commits_commit_time" ON commits USING btree (commit_time);
CREATE INDEX "idx_commits_exercise_cases_commit_id" ON commits_exercise_cases USING btree (commit_id);
CREATE INDEX "idx_commits_exercise_cases_exercise_case_id" ON commits_exercise_cases USING btree (exercise_case_id);
CREATE INDEX "idx_commits_exercise_id" ON commits USING btree (exercise_id);
CREATE INDEX "idx_commits_status" ON commits USING btree (status);
CREATE INDEX "idx_commits_user_email" ON commits USING btree (user_email);
CREATE INDEX "idx_commits_user_email_hash" ON commits USING hash (user_email);
CREATE INDEX "idx_enrollments_offering_id" ON enrollments USING btree (offering_id);
CREATE INDEX "idx_enrollments_user_email" ON enrollments USING btree (user_email);
CREATE INDEX "idx_exercise_cases_exercise_id" ON exercise_cases USING btree (exercise_id);
CREATE INDEX "idx_exercises_deadline" ON exercises USING btree (deadline);
CREATE INDEX "idx_exercises_offering_id" ON exercises USING btree (offering_id);
CREATE INDEX "idx_exercises_open_date" ON exercises USING btree (open_date);
CREATE INDEX "idx_mail_logs_hash" ON mail_logs USING hash (hash);

RESET role;

-- Privileges (same end state as the per-object ACL section of the dump)
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM public, runcodes;
GRANT ALL ON ALL TABLES IN SCHEMA public TO runcodes, postgres;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM public, runcodes;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO runcodes, postgres;
