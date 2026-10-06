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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func reader(files map[string]string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		if s, ok := files[p]; ok {
			return []byte(s), nil
		}
		return nil, fs.ErrNotExist
	}
}

func TestDefaultMemoryLimitIsAnEighthOfTheCgroupLimit(t *testing.T) {
	require.EqualValues(t, 4<<30/8, defaultMemoryLimit(reader(map[string]string{
		"/sys/fs/cgroup/memory.max": "4294967296\n",
	})))
}

func TestDefaultMemoryLimitFallsBackToMemTotal(t *testing.T) {
	require.EqualValues(t, int64(16<<20)*1024/8, defaultMemoryLimit(reader(map[string]string{
		"/sys/fs/cgroup/memory.max": "max\n",
		"/proc/meminfo":             "MemTotal:       16777216 kB\nMemFree: 1 kB\n",
	})))
}

func TestDefaultMemoryLimitHasAFloor(t *testing.T) {
	require.EqualValues(t, 256<<20, defaultMemoryLimit(reader(map[string]string{
		"/sys/fs/cgroup/memory.max": "536870912\n",
	})))
	require.EqualValues(t, 256<<20, defaultMemoryLimit(reader(nil)))
}

func TestResolveFillsDefaults(t *testing.T) {
	dir := t.TempDir()
	index := filepath.Join(dir, "set.par2")
	require.NoError(t, os.WriteFile(index, nil, 0o644))
	r, err := Options{}.resolve(index)
	require.NoError(t, err)
	require.Equal(t, dir, r.dir)
	require.Equal(t, runtime.GOMAXPROCS(0), r.threads)
	require.Equal(t, 2, r.fileThreads)
	require.Equal(t, 250*time.Millisecond, r.pollEvery)
	require.GreaterOrEqual(t, r.memoryLimit, int64(256<<20))
}

func TestResolveRefusesBadOptions(t *testing.T) {
	dir := t.TempDir()
	index := filepath.Join(dir, "set.par2")
	require.NoError(t, os.WriteFile(index, nil, 0o644))
	outside := filepath.Join(t.TempDir(), "x.rar")
	require.NoError(t, os.WriteFile(outside, nil, 0o644))
	for name, tc := range map[string]struct {
		index string
		opts  Options
	}{
		"missing index":     {filepath.Join(dir, "nope.par2"), Options{}},
		"index is a dir":    {dir, Options{}},
		"extra outside dir": {index, Options{ExtraFiles: []string{outside}}},
		"missing extra":     {index, Options{ExtraFiles: []string{filepath.Join(dir, "gone")}}},
		"negative memory":   {index, Options{MemoryLimit: -1}},
		"negative threads":  {index, Options{Threads: -1}},
		"negative poll":     {index, Options{PollEvery: -time.Second}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := tc.opts.resolve(tc.index)
			require.True(t, errors.Is(err, ErrInvalidOptions), "%v", err)
		})
	}
}
