#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "monitor.h"

static MonitorParams DEFAULT_PARAMS = {
    NULL, "monitored.out", "monitored.err", NULL, 0, 0, 0, NULL};

/*
 * Parses arguments and store extracted monitor parameters in 'params'.
 *
 * Returns NULL on success (no error message); otherwise returns a string
 * describing the error.
 */
const char *parse_args(int argc, char *argv[], MonitorParams *params);

/*
 * Print usage info.
 */
void usage();

/*
 * Prints error messages in a standard way.
 */
void print_error(const char *msg);

static const char *parse_limit(const char *text, unsigned long *limit) {
  char *end = NULL;

  if (text == NULL || *text < '0' || *text > '9')
    return "incorrect limit parameter";

  errno = 0;
  unsigned long parsed = strtoul(text, &end, 10);
  if (errno == ERANGE || end == text || *end != '\0')
    return "incorrect limit parameter";

  *limit = parsed;
  return NULL;
}

const char *parse_args(int argc, char *argv[], MonitorParams *params) {
  if (params == NULL)
    return "'params' not set";

  int opt;
  unsigned long limit;
  const char *limit_error;
  while ((opt = getopt(argc, argv, "i:o:e:r:f:m:s:c:h")) != -1) {
    switch (opt) {
    case 'h':
      usage();
      exit(0);
    case 'i':
      params->in_fname = optarg;
      break;
    case 'o':
      params->out_fname = optarg;
      break;
    case 'e':
      params->err_fname = optarg;
      break;
    case 'r':
      params->report_fname = optarg;
      break;
    case 'f':
      if ((limit_error = parse_limit(optarg, &limit)) != NULL)
        return limit_error;
      params->fs_limit = limit;
      break;
    case 'm':
      if ((limit_error = parse_limit(optarg, &limit)) != NULL)
        return limit_error;
      params->ms_limit = limit;
      break;
    case 's':
      if ((limit_error = parse_limit(optarg, &limit)) != NULL)
        return limit_error;
      params->stack_limit = limit;
      break;
    case 'c':
      params->command = optarg;
      break;
    default:
      return "incorrect parameters";
    }
  }

  if (params->command == NULL || params->command[0] == '\0')
    return "missing -c option";
  if (strlen(params->command) >= COMMAND_MAX)
    return "command is too long";
  return NULL;
}

void usage() {
  fprintf(stderr,
          "Usage: monitor [options]"
          "\n\t-i INFILE"
          "\n\t-o OUTFILE          \t Default: %s"
          "\n\t-e ERRFILE          \t Default: %s"
          "\n\t-r REPORTFILE       \t Default: stdout"
          "\n\t-f MAX_FILE_BYTES   \t 0: no limit"
          "\n\t-m MAX_MEM_BYTES    \t 0: no limit"
          "\n\t-s MAX_STACK_BYTES  \t 0: no limit"
          "\n\t-c COMMAND\n",
          DEFAULT_PARAMS.out_fname, DEFAULT_PARAMS.err_fname);
}

void print_error(const char *msg) {
  fprintf(stderr, "monitor: error: %s\n", msg);
}

int main(int argc, char *argv[]) {
  MonitorParams params = DEFAULT_PARAMS;

  const char *error_msg = parse_args(argc, argv, &params);
  if (error_msg) {
    print_error(error_msg);
    exit(EXIT_FAILURE);
  }

  MonitorStats stats;
  if (run_monitor(&params, &stats) != 0) {
    print_error("could not start the command");
    exit(EXIT_FAILURE);
  }

  char report[512];
  if (format_report(&stats, report, sizeof(report)) < 0) {
    print_error("could not format the report");
    exit(EXIT_FAILURE);
  }

  if (!params.report_fname) {
    /* No report file was requested: keep the original behaviour and print it
       to stdout, which the caller may be redirecting. */
    fputs(report, stdout);
    return 0;
  }

  if (!write_report_atomic(params.report_fname, report)) {
    print_error("could not write the report");
    exit(EXIT_FAILURE);
  }

  return 0;
}
