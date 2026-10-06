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

package par2

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Options tune a run. The zero value is usable.
type Options struct {
	// Dir is the directory the set's file names are relative to; it
	// defaults to the index file's directory.
	Dir string
	// ExtraFiles are scanned by content, so files on disk under other
	// names (obfuscated, renamed) are matched. Each must be under Dir.
	ExtraFiles []string
	// MemoryLimit is in bytes; 0 is 1/8 of the cgroup's memory limit (or
	// of MemTotal), at least 256 MiB.
	MemoryLimit int64
	// Threads is the repair's compute threads; 0 is GOMAXPROCS.
	Threads int
	// FileThreads is files verified at once; 0 is 2, par2's default.
	FileThreads int
	// Purge makes Repair delete the .1 backups and the par2 files once the
	// set verifies. Verify ignores it.
	Purge bool
	// Progress, if set, is called from the calling goroutine every
	// PollEvery while the job runs.
	Progress func(Progress)
	// PollEvery defaults to 250ms.
	PollEvery time.Duration
}

type resolved struct {
	index, dir  string
	extras      []string
	memoryLimit int64
	threads     int
	fileThreads int
	purge       bool
	pollEvery   time.Duration
}

const memoryFloor = 256 << 20

func (o Options) resolve(index string) (resolved, error) {
	bad := func(format string, a ...any) (resolved, error) {
		return resolved{}, fmt.Errorf("%w: %s", ErrInvalidOptions, fmt.Sprintf(format, a...))
	}
	abs, err := filepath.Abs(index)
	if err != nil {
		return bad("index %q: %v", index, err)
	}
	if st, err := os.Stat(abs); err != nil || !st.Mode().IsRegular() {
		return bad("index %q is not a regular file", index)
	}
	r := resolved{index: abs, purge: o.Purge}
	r.dir = o.Dir
	if r.dir == "" {
		r.dir = filepath.Dir(abs)
	}
	if r.dir, err = filepath.Abs(r.dir); err != nil {
		return bad("dir %q: %v", o.Dir, err)
	}
	if st, err := os.Stat(r.dir); err != nil || !st.IsDir() {
		return bad("dir %q is not a directory", r.dir)
	}
	for _, e := range o.ExtraFiles {
		a, err := filepath.Abs(e)
		if err != nil {
			return bad("extra file %q: %v", e, err)
		}
		rel, err := filepath.Rel(r.dir, a)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return bad("extra file %q is outside %q", e, r.dir)
		}
		if st, err := os.Stat(a); err != nil || !st.Mode().IsRegular() {
			return bad("extra file %q is not a regular file", e)
		}
		r.extras = append(r.extras, a)
	}
	switch {
	case o.MemoryLimit < 0:
		return bad("MemoryLimit %d is negative", o.MemoryLimit)
	case o.Threads < 0:
		return bad("Threads %d is negative", o.Threads)
	case o.FileThreads < 0:
		return bad("FileThreads %d is negative", o.FileThreads)
	case o.PollEvery < 0:
		return bad("PollEvery %v is negative", o.PollEvery)
	}
	r.memoryLimit = o.MemoryLimit
	if r.memoryLimit == 0 {
		r.memoryLimit = defaultMemoryLimit(os.ReadFile)
	}
	r.threads = o.Threads
	if r.threads == 0 {
		r.threads = runtime.GOMAXPROCS(0)
	}
	r.fileThreads = o.FileThreads
	if r.fileThreads == 0 {
		r.fileThreads = 2
	}
	r.pollEvery = o.PollEvery
	if r.pollEvery == 0 {
		r.pollEvery = 250 * time.Millisecond
	}
	return r, nil
}

// defaultMemoryLimit is 1/8 of the cgroup v2 memory limit, or of MemTotal
// when there is none, at least 256 MiB. par2's own default reads the
// host's memory, which in a pod can exceed the container's limit.
func defaultMemoryLimit(read func(string) ([]byte, error)) int64 {
	total := int64(0)
	if b, err := read("/sys/fs/cgroup/memory.max"); err == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64); err == nil {
			total = n
		}
	}
	if total == 0 {
		if b, err := read("/proc/meminfo"); err == nil {
			s := bufio.NewScanner(bytes.NewReader(b))
			for s.Scan() {
				f := strings.Fields(s.Text())
				if len(f) >= 2 && f[0] == "MemTotal:" {
					if kb, err := strconv.ParseInt(f[1], 10, 64); err == nil {
						total = kb * 1024
					}
				}
			}
		}
	}
	return max(total/8, memoryFloor)
}
