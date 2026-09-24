#ifndef MONITOR_H
#define MONITOR_H

#include <stdio.h>

typedef struct {
  const char *in_fname;
  const char *out_fname;
  const char *err_fname;
  /*
   * Where the report is written. When NULL the report goes to stdout, which is
   * how the monitor behaved before -r existed.
   */
  const char *report_fname;
  unsigned long fs_limit;    /* file size limit */
  unsigned long ms_limit;    /* memory (address space) limit */
  unsigned long stack_limit; /* stack limit, 0 for the default */
  const char *command;
} MonitorParams;

typedef struct {
  int exit_status;
  int signal;
  float time_sec;
} MonitorStats;

/*
 * Returns a string with the name of the signal (SIGSOMETHING) or UNKNOWN if the
 * signal is unkown.
 */
const char *signal_name(int signal);

/*
 * Runs a monitored child command using the given parameters, storing execution
 * info in `stats`.
 *
 * The command runs in its own process group, and anything left in that group is
 * killed before run_monitor returns, so a program cannot outlive its case and
 * then rewrite the files the judge grades.
 */
int run_monitor(const MonitorParams *params, MonitorStats *stats);

/*
 * Renders the report for `stats` into `buf` (NUL-terminated) and returns its
 * length, or -1 when the buffer is too small.
 */
int format_report(const MonitorStats *stats, char *buf, size_t buflen);

/*
 * Writes `content` to `path`, replacing whatever is there.
 *
 * The path is chosen by the harness but is inside a directory the graded
 * program can write, so the content goes to a fresh temporary file first and is
 * renamed into place: rename(2) replaces a pre-created file or symlink instead
 * of following it, and the temporary name is opened with O_EXCL|O_NOFOLLOW.
 *
 * Returns 1 on success, 0 on failure.
 */
int write_report_atomic(const char *path, const char *content);

#endif // MONITOR_H
