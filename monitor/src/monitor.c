#include "monitor.h"

#include <errno.h>
#include <fcntl.h>
#include <limits.h>
#include <signal.h>
#include <stdlib.h>
#include <string.h>
#include <sys/resource.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

#define COMMAND_MAX 2000

const char *signal_name(int signal) {
  static char *signal_list[] = {
      "UNKNOWN",   /* 0 */
      "SIGHUP",    /* 1 */
      "SIGINT",    /* 2 */
      "SIGQUIT",   /* 3 */
      "SIGILL",    /* 4 */
      "SIGTRAP",   /* 5 */
      "SIGABRT",   /* 6 */
      "SIGBUS",    /* 7 */
      "SIGFPE",    /* 8 */
      "SIGKILL",   /* 9 */
      "SIGUSR1",   /* 10 */
      "SIGSEGV",   /* 11 */
      "SIGUSR2",   /* 12 */
      "SIGPIPE",   /* 13 */
      "SIGALRM",   /* 14 */
      "SIGTERM",   /* 15 */
      "SIGSTKFLT", /* 16 */
      "SIGCHLD",   /* 17 */
      "SIGCONT",   /* 18 */
      "SIGSTOP",   /* 19 */
      "SIGTSTP",   /* 20 */
      "SIGTTIN",   /* 21 */
      "SIGTTOU",   /* 22 */
      "SIGURG",    /* 23 */
      "SIGXCPU",   /* 24 */
      "SIGXFSZ",   /* 25 */
      "SIGVTALRM", /* 26 */
      "SIGPROF",   /* 27 */
      "SIGWINCH",  /* 28 */
      "SIGIO",     /* 29 */
      "SIGPWR",    /* 30 */
      "SIGSYS",    /* 31 */
  };

  if (signal == -1)
    return ""; /* No signal */

  if (signal < 0 || signal >= (int)(sizeof(signal_list) / sizeof(char *)))
    signal = 0; /* Unknown signal */
  return signal_list[signal];
}

static float elapsed_secs(struct timespec *ts_start, struct timespec *ts_end) {
  float delta_sec = ts_end->tv_sec - ts_start->tv_sec;
  float delta_nsec = ts_end->tv_nsec - ts_start->tv_nsec;
  return delta_sec + delta_nsec * 1e-9f;
}

static char **parse_args(char *command) {
  char **args = NULL, *arg;
  int i;

  arg = strtok(command, " \t");
  for (i = 0; arg != NULL; i++) {
    args = (char **)realloc(args, sizeof(char *) * (i + 2));
    args[i] = arg;
    arg = strtok(NULL, " \t");
  }
  args[i] = NULL;

  return args;
}

static int set_limits(const MonitorParams *params) {
  /* File size limit */
  if (params->fs_limit > 0) {
    struct rlimit fs_limit;

    fs_limit.rlim_cur = fs_limit.rlim_max = params->fs_limit;
    if (setrlimit(RLIMIT_FSIZE, &fs_limit) < 0)
      return 0;
  }

  /* Memory size limit */
  if (params->ms_limit > 0) {
    struct rlimit ms_limit;

    ms_limit.rlim_cur = ms_limit.rlim_max = params->ms_limit;
    if (setrlimit(RLIMIT_AS, &ms_limit) < 0)
      return 0;
  }

  /* Stack limit. It is left alone when unset, because shrinking it breaks
     runtimes that need a large stack (the JVM, for one). */
  if (params->stack_limit > 0) {
    struct rlimit stack_limit;

    stack_limit.rlim_cur = stack_limit.rlim_max = params->stack_limit;
    if (setrlimit(RLIMIT_STACK, &stack_limit) < 0)
      return 0;
  }

  return 1;
}

static int reopen_streams(const MonitorParams *params) {
  if (params->in_fname)
    if (!freopen(params->in_fname, "r", stdin))
      return 0;
  if (!freopen(params->out_fname, "w", stdout))
    return 0;
  if (!freopen(params->err_fname, "w", stderr))
    return 0;
  return 1;
}

static void exec_cmd(const MonitorParams *params) {
  char buf[COMMAND_MAX + 1];
  strncpy(buf, params->command, COMMAND_MAX);
  char **args = parse_args(buf);

  if (!set_limits(params)) {
    fprintf(stderr, "monitor: Failed to set process limits.\n");
    exit(255);
  }

  if (!reopen_streams(params)) {
    fprintf(stderr, "monitor: Failed to initialize streams.\n");
    exit(255);
  }

  if (execvp(args[0], args) == -1) {
    fprintf(stderr, "monitor: exec failed\n");
    exit(255);
  }
}

static void kill_group(pid_t pgid) {
  /* Everything the command started shares its process group, including a
     program that forked helpers for itself. Killing the group here - after the
     direct child was reaped, before the report is written - means nothing of
     the graded program is still able to touch the artifacts being collected.

     A process that puts itself in a new session (setsid) escapes this group;
     the harness kills what is left of the container's processes as soon as the
     monitor returns, so the remaining window is the few milliseconds between
     the report being renamed into place and that sweep. */
  if (kill(-pgid, SIGKILL) != 0 && errno != ESRCH) {
    fprintf(stderr, "monitor: could not kill the command's process group\n");
  }
}

static void trace_child(pid_t child_pid, MonitorStats *stats) {
  int status;

  struct timespec ts_start, ts_end;
  clock_gettime(CLOCK_MONOTONIC, &ts_start);
  for (;;) {
    pid_t pid = waitpid(child_pid, &status, 0);
    if (pid == -1) {
      if (errno == EINTR)
        continue;

      // error (ECHILD means the child was already reaped elsewhere)
      fprintf(stderr, "monitor: error: waitpid()\n");
      break;
    }
    if (WIFEXITED(status)) {
      clock_gettime(CLOCK_MONOTONIC, &ts_end);
      stats->time_sec = elapsed_secs(&ts_start, &ts_end);
      stats->exit_status = WEXITSTATUS(status);
      break;
    } else if (WIFSIGNALED(status)) {
      clock_gettime(CLOCK_MONOTONIC, &ts_end);
      stats->time_sec = elapsed_secs(&ts_start, &ts_end);
      if (WCOREDUMP(status)) {
      } else
        stats->signal = WTERMSIG(status);
      break;
    }
  }
}

static void init_stats(MonitorStats *stats) {
  stats->exit_status = -1;
  stats->signal = -1;
  stats->time_sec = -1.0f;
}

int run_monitor(const MonitorParams *params, MonitorStats *stats) {
  init_stats(stats);

  pid_t pid = fork();
  switch (pid) {
  case -1:
    // failure
    return -1;
  case 0:
    // child: lead its own process group so the monitor can clean up
    // everything it starts without signalling itself
    setpgid(0, 0);
    exec_cmd(params);
    break;
  default:
    // Also done from the parent to close the race with exec().
    setpgid(pid, pid);
    trace_child(pid, stats);
    kill_group(pid);
    break;
  }

  return 0;
}

int format_report(const MonitorStats *stats, char *buf, size_t buflen) {
  int written =
      snprintf(buf, buflen,
               "exit_status=%d\n"
               "signal=%s\n"
               "time=%f\n",
               stats->exit_status, signal_name(stats->signal), stats->time_sec);

  if (written < 0 || (size_t)written >= buflen)
    return -1;

  return written;
}

int write_report_atomic(const char *path, const char *content) {
  char tmp[PATH_MAX];
  size_t len = strlen(content);
  int fd = -1;

  /* The directory and the file name are known to the graded program, so a
     pre-created file or symlink at the temporary name must not be reused: the
     open is exclusive, and a collision just picks another name. */
  for (int attempt = 0; attempt < 8 && fd < 0; attempt++) {
    if (snprintf(tmp, sizeof(tmp), "%s.tmp.%d.%d", path, (int)getpid(),
                 attempt) >= (int)sizeof(tmp))
      return 0;

    fd = open(tmp, O_WRONLY | O_CREAT | O_EXCL | O_NOFOLLOW, 0644);
    if (fd < 0 && errno != EEXIST)
      return 0;
  }

  if (fd < 0)
    return 0;

  size_t written = 0;
  while (written < len) {
    ssize_t n = write(fd, content + written, len - written);
    if (n < 0) {
      if (errno == EINTR)
        continue;

      close(fd);
      unlink(tmp);
      return 0;
    }
    written += (size_t)n;
  }

  if (close(fd) != 0) {
    unlink(tmp);
    return 0;
  }

  /* Replaces a pre-existing file or symlink rather than writing through it. */
  if (rename(tmp, path) != 0) {
    unlink(tmp);
    return 0;
  }

  return 1;
}
