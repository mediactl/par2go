/*
Copyright 2026 The Clustarr Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/
#ifndef PAR2GO_H
#define PAR2GO_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define P2_EXPORT __attribute__((visibility("default")))

#define P2_ABI_VERSION 1
#define P2_NAME_MAX 4096

/* p2_run's return: 0-8 are par2cmdline-turbo's Par2::Result. */
enum {
  P2_SUCCESS = 0,
  P2_REPAIR_POSSIBLE = 1,
  P2_REPAIR_NOT_POSSIBLE = 2,
  P2_INVALID_ARGS = 3,
  P2_INSUFFICIENT_CRITICAL_DATA = 4,
  P2_REPAIR_FAILED = 5,
  P2_FILE_IO_ERROR = 6,
  P2_LOGIC_ERROR = 7,
  P2_MEMORY_ERROR = 8,
  P2_CANCELLED = 100,
  P2_INVALID = 101
};

enum { P2_PHASE_VERIFYING = 1, P2_PHASE_REPAIRING = 2 };
enum { P2_FILE_COMPLETE = 1, P2_FILE_RENAMED = 2, P2_FILE_DAMAGED = 3, P2_FILE_MISSING = 4 };
enum { P2_SIZEOF_PROGRESS = 1, P2_SIZEOF_COUNTS = 2, P2_SIZEOF_FILE = 3 };

/* Every field is 8 bytes wide or a byte array last, so Go mirrors it exactly. */
typedef struct {
  int64_t phase;
  int64_t per_mille;
  int64_t file_truncated;
  char file[P2_NAME_MAX];
} p2_progress;

typedef struct {
  int64_t repair_attempted;
  int64_t block_size;
  int64_t data_blocks;
  int64_t recovery_blocks;
  int64_t recoverable_files;
  int64_t other_files;
  int64_t complete_files;
  int64_t renamed_files;
  int64_t damaged_files;
  int64_t missing_files;
  int64_t available_blocks;
  int64_t missing_blocks;
  int64_t files;
  char set_id[40];
} p2_counts;

typedef struct {
  int64_t state;
  int64_t blocks_available;
  int64_t blocks_total;
  int64_t name_truncated;
  char name[P2_NAME_MAX];
} p2_file_result;

typedef struct p2_job p2_job;

P2_EXPORT int32_t p2_abi_version(void);
P2_EXPORT uint64_t p2_sizeof(int32_t which);

P2_EXPORT p2_job *p2_new(const char *index, const char *basepath, int64_t memory_limit,
                         int32_t threads, int32_t file_threads, int32_t purge);
P2_EXPORT int32_t p2_add_extra(p2_job *job, const char *path);
P2_EXPORT int32_t p2_run(p2_job *job, int32_t repair);
P2_EXPORT void p2_progress_read(p2_job *job, p2_progress *out);
P2_EXPORT void p2_cancel(p2_job *job);
P2_EXPORT int32_t p2_counts_read(p2_job *job, p2_counts *out);
P2_EXPORT int32_t p2_file_read(p2_job *job, int32_t index, p2_file_result *out);
P2_EXPORT uint64_t p2_log_read(p2_job *job, char *buf, uint64_t n);
P2_EXPORT void p2_free(p2_job *job);

#ifdef __cplusplus
}
#endif
#endif
