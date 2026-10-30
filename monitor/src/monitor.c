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

static char **parse_args(char *command, const char **error) {
  char **args = NULL, *arg;
  size_t count = 0;

  *error = NULL;

  if (command == NULL || *command == '\0') {
    *error = "empty command";
    return NULL;
  }

  for (arg = strtok(command, " \t"); arg != NULL; arg = strtok(NULL, " \t")) {
    char **grown = (char **)realloc(args, sizeof(char *) * (count + 2));
    if (grown == NULL) {
      free(args);
      *error = "out of memory";
      return NULL;
    }
    args = grown;
    args[count++] = arg;
  }

  if (count == 0) {
    free(args);
    *error = "empty command";
    return NULL;
  }

  args[count] = NULL;
  return args;
}

static int set_limits(const MonitorParams *params) {
  /* Core dumps are never useful for grading and can fill the workspace at disk
     speed. Disable them for every run, regardless of the exercise limits. */
  {
    struct rlimit core_limit;

    core_limit.rlim_cur = core_limit.rlim_max = 0;
    if (setrlimit(RLIMIT_CORE, &core_limit) < 0)
      return 0;
  }

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
  const char *error = NULL;

  if (params->command == NULL || strlen(params->command) >= sizeof(buf)) {
    fprintf(stderr, "monitor: Failed to initialize command.\n");
    exit(255);
  }
  snprintf(buf, sizeof(buf), "%s", params->command);

  char **args = parse_args(buf, &error);
  if (args == NULL) {
    fprintf(stderr, "monitor: %s\n", error != NULL ? error : "invalid command");
    exit(255);
  }

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
    free(args);
    exit(255);
  }
}

/*
 * read_cpu_v2_file reads the `usage_usec` field of a cgroup v2 cpu.stat file.
 * It returns -1 when the file cannot be read.
 */
static long long read_cpu_v2_file(const char *path) {
  FILE *f = fopen(path, "r");
  if (f == NULL)
    return -1;

  char key[64];
  long long value;
  while (fscanf(f, "%63s %lld", key, &value) == 2) {
    if (strcmp(key, "usage_usec") == 0) {
      fclose(f);
      return value * 1000;
    }
  }
  fclose(f);
  return -1;
}

/*
 * read_cpu_usage_ns returns the aggregate CPU time charged to this process's
 * cgroup, in nanoseconds, or -1 when the host does not expose accounting (for
 * example a restricted cgroup namespace).
 *
 * /proc/self/cgroup is consulted instead of assuming the process is at the
 * cgroup root: a container whose cgroup namespace was not made private still
 * lives below the host hierarchy.
 */
static long long read_cpu_usage_ns(void) {
  FILE *cg = fopen("/proc/self/cgroup", "r");
  if (cg != NULL) {
    char line[PATH_MAX + 64];
    char rel[PATH_MAX];

    while (fgets(line, sizeof(line), cg) != NULL) {
      if (sscanf(line, "0::%4095s", rel) == 1) {
        char path[PATH_MAX];
        int n;
        if (strcmp(rel, "/") == 0)
          n = snprintf(path, sizeof(path), "/sys/fs/cgroup/cpu.stat");
        else
          n = snprintf(path, sizeof(path), "/sys/fs/cgroup%s/cpu.stat", rel);

        if (n > 0 && (size_t)n < sizeof(path)) {
          fclose(cg);
          return read_cpu_v2_file(path);
        }
      }
    }
    fclose(cg);
  }

  /* Some containers mount only the v1 cpuacct controller. */
  FILE *f = fopen("/sys/fs/cgroup/cpuacct/cpuacct.usage", "r");
  if (f != NULL) {
    long long value;
    if (fscanf(f, "%lld", &value) == 1) {
      fclose(f);
      return value;
    }
    fclose(f);
  }

  return -1;
}

static float rusage_cpu_secs(const struct rusage *usage) {
  return (float)usage->ru_utime.tv_sec + (float)usage->ru_utime.tv_usec / 1e6f +
         (float)usage->ru_stime.tv_sec + (float)usage->ru_stime.tv_usec / 1e6f;
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
  struct rusage usage = {0};

  struct timespec ts_start, ts_end;
  clock_gettime(CLOCK_MONOTONIC, &ts_start);
  for (;;) {
    pid_t pid = wait4(child_pid, &status, 0, &usage);
    if (pid == -1) {
      if (errno == EINTR)
        continue;

      // error (ECHILD means the child was already reaped elsewhere)
      fprintf(stderr, "monitor: error: wait4()\n");
      break;
    }
    if (WIFEXITED(status) || WIFSIGNALED(status)) {
      clock_gettime(CLOCK_MONOTONIC, &ts_end);
      stats->time_sec = elapsed_secs(&ts_start, &ts_end);
      stats->child_cpu_sec = rusage_cpu_secs(&usage);
      if (WIFSIGNALED(status)) {
        /* Report the terminating signal whether or not a core dump was
           produced. A core dump does not mean the process exited cleanly, and
           leaving `signal` unset let a crash (SIGSEGV, for one) look like a
           clean run to callers that only read `signal`. */
        stats->signal = WTERMSIG(status);
      } else if (WIFEXITED(status)) {
        stats->exit_status = WEXITSTATUS(status);
      }
      break;
    }
  }
}

static void init_stats(MonitorStats *stats) {
  stats->exit_status = -1;
  stats->signal = -1;
  stats->time_sec = -1.0f;
  stats->cpu_sec = -1.0f;
  stats->child_cpu_sec = -1.0f;
}

int run_monitor(const MonitorParams *params, MonitorStats *stats) {
  init_stats(stats);
  long long cpu_before = read_cpu_usage_ns();

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

    long long cpu_after = read_cpu_usage_ns();
    if (cpu_before >= 0 && cpu_after >= cpu_before) {
      stats->cpu_sec = (float)(cpu_after - cpu_before) / 1e9f;
    } else {
      stats->cpu_sec = stats->child_cpu_sec;
    }
    break;
  }

  return 0;
}

int format_report(const MonitorStats *stats, char *buf, size_t buflen) {
  int written =
      snprintf(buf, buflen,
               "exit_status=%d\n"
               "signal=%s\n"
               "time=%f\n"
               "cpu_time=%f\n",
               stats->exit_status, signal_name(stats->signal), stats->time_sec,
               stats->cpu_sec);

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
