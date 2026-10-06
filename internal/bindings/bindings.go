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

package bindings

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// ErrUnavailable reports that libpar2go.so could not be loaded or is not
// the version this module was built against.
var ErrUnavailable = errors.New("par2go: libpar2go.so unavailable")

// The shim's functions, registered by Load. They are nil until Load succeeds.
var (
	ABIVer       func() int32
	Sizeof       func(which int32) uint64
	New          func(index, basepath string, memoryLimit int64, threads, fileThreads, purge int32) uintptr
	AddExtra     func(job uintptr, path string) int32
	Start        func(job uintptr, repair int32) int32
	ActiveJobs   func() int32
	ReadProgress func(job uintptr, out *Progress)
	Cancel       func(job uintptr)
	ReadCounts   func(job uintptr, out *Counts) int32
	ReadFile     func(job uintptr, index int32, out *File) int32
	ReadLog      func(job uintptr, buf *byte, n uint64) uint64
	Free         func(job uintptr)
)

type symbol struct {
	name string
	fn   any
}

// symbols lists every p2_* function Load registers.
func symbols() []symbol {
	return []symbol{
		{"p2_abi_version", &ABIVer},
		{"p2_sizeof", &Sizeof},
		{"p2_new", &New},
		{"p2_add_extra", &AddExtra},
		{"p2_start", &Start},
		{"p2_active_jobs", &ActiveJobs},
		{"p2_progress_read", &ReadProgress},
		{"p2_cancel", &Cancel},
		{"p2_counts_read", &ReadCounts},
		{"p2_file_read", &ReadFile},
		{"p2_log_read", &ReadLog},
		{"p2_free", &Free},
	}
}

var (
	once    sync.Once
	loadErr error
	path    string
)

// Candidates is the search order: $PAR2GO_LIB when set, then the bare
// soname for the dynamic loader's own search.
func Candidates(getenv func(string) string) []string {
	var c []string
	if p := getenv("PAR2GO_LIB"); p != "" {
		c = append(c, p)
	}
	return append(c, "libpar2go.so")
}

// Load loads the library once; every later call returns the first result.
func Load() error {
	once.Do(func() { path, loadErr = load(Candidates(os.Getenv)) })
	return loadErr
}

// Path is the file the library was mapped from, or "" before a successful Load.
func Path() string { return path }

func load(cands []string) (string, error) {
	var tried []string
	for _, c := range cands {
		h, err := purego.Dlopen(c, purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			tried = append(tried, fmt.Sprintf("%s (%v)", c, err))
			continue
		}
		if err := register(h); err != nil {
			return "", fmt.Errorf("%w: %s: %v", ErrUnavailable, c, err)
		}
		if v := ABIVer(); v != ABIVersion {
			return "", fmt.Errorf("%w: %s has ABI %d, want %d", ErrUnavailable, c, v, ABIVersion)
		}
		if err := checkSizes(); err != nil {
			return "", fmt.Errorf("%w: %s: %v", ErrUnavailable, c, err)
		}
		return mappedPath(), nil
	}
	return "", fmt.Errorf("%w: tried %s", ErrUnavailable, strings.Join(tried, "; "))
}

func register(h uintptr) error {
	for _, s := range symbols() {
		addr, err := purego.Dlsym(h, s.name)
		if err != nil {
			return fmt.Errorf("symbol %s: %w", s.name, err)
		}
		purego.RegisterFunc(s.fn, addr)
	}
	return nil
}

func checkSizes() error {
	for _, c := range []struct {
		which int32
		name  string
		want  uintptr
	}{
		{sizeofProgress, "p2_progress", unsafe.Sizeof(Progress{})},
		{sizeofCounts, "p2_counts", unsafe.Sizeof(Counts{})},
		{sizeofFile, "p2_file_result", unsafe.Sizeof(File{})},
	} {
		if got := Sizeof(c.which); got != uint64(c.want) {
			return fmt.Errorf("sizeof(%s) is %d in C, %d in Go", c.name, got, c.want)
		}
	}
	return nil
}

// mappedPath finds the library's file in /proc/self/maps, which also
// resolves a bare soname to the file the loader chose.
func mappedPath() string {
	b, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if i := strings.Index(line, "/"); i >= 0 && strings.Contains(line[i:], "libpar2go") {
			return strings.TrimSpace(line[i:])
		}
	}
	return ""
}
