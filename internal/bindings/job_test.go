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

package bindings_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mediactl/par2go/internal/bindings"
	"github.com/mediactl/par2go/internal/testlib"
)

type raw struct {
	code   int32
	counts bindings.Counts
	files  []bindings.File
	log    string
}

func runRaw(t *testing.T, dir string, repair bool, extras ...string) raw {
	t.Helper()
	job := bindings.New(filepath.Join(dir, "set.par2"), dir, 64<<20, int32(runtime.GOMAXPROCS(0)), 2, 0)
	require.NotZero(t, job)
	defer bindings.Free(job)
	for _, e := range extras {
		require.Zero(t, bindings.AddExtra(job, filepath.Join(dir, e)))
	}
	r := 0
	if repair {
		r = 1
	}
	var out raw
	out.code = runJob(t, job, int32(r))
	require.Zero(t, bindings.ReadCounts(job, &out.counts))
	for i := int32(0); i < int32(out.counts.Files); i++ {
		var f bindings.File
		require.Zero(t, bindings.ReadFile(job, i, &f))
		out.files = append(out.files, f)
	}
	buf := make([]byte, 4096)
	n := bindings.ReadLog(job, &buf[0], uint64(len(buf)))
	out.log = string(buf[:min(n, uint64(len(buf)))])
	return out
}

// The working directory is the package dir, not the set's: this fails if
// the shim leaves Par2Repairer's basepath empty (upstream fact 1).
func TestShimVerifiesAnIntactSetOutsideTheWorkingDirectory(t *testing.T) {
	testlib.RequireLib(t)
	r := runRaw(t, testlib.CopyCase(t, "intact"), false)
	require.EqualValues(t, bindings.Success, r.code, r.log)
	require.EqualValues(t, 2, r.counts.CompleteFiles)
	require.EqualValues(t, 13, r.counts.DataBlocks)
	require.EqualValues(t, 2, r.counts.RecoveryBlocks)
	require.EqualValues(t, 1024, r.counts.BlockSize)
	require.EqualValues(t, 0, r.counts.RepairAttempted)
	require.Len(t, bindings.CString(r.counts.SetID[:]), 32)
	require.Len(t, r.files, 2)
	for _, f := range r.files {
		require.EqualValues(t, bindings.FileComplete, f.State, bindings.CString(f.Name[:]))
		require.Equal(t, f.BlocksTotal, f.BlocksAvailable)
	}
	require.ElementsMatch(t, []string{"a.bin", "b.bin"},
		[]string{bindings.CString(r.files[0].Name[:]), bindings.CString(r.files[1].Name[:])})
}

func TestShimReportsTheBlocksAnUnrepairableSetLacks(t *testing.T) {
	testlib.RequireLib(t)
	r := runRaw(t, testlib.CopyCase(t, "unrepairable"), false)
	require.EqualValues(t, bindings.RepairNotPossible, r.code, r.log)
	require.EqualValues(t, 5, r.counts.MissingBlocks)
	require.EqualValues(t, 2, r.counts.RecoveryBlocks)
	require.EqualValues(t, 1, r.counts.DamagedFiles)
	require.Contains(t, r.log, "You need 3 more recovery blocks")
}

func TestShimRepairsInOnePassOverOneLoad(t *testing.T) {
	testlib.RequireLib(t)
	dir := testlib.CopyCase(t, "repairable")
	r := runRaw(t, dir, true)
	require.EqualValues(t, bindings.Success, r.code, r.log)
	require.EqualValues(t, 1, r.counts.RepairAttempted)
	require.EqualValues(t, 1, r.counts.DamagedFiles, "counts are the verify-time snapshot")
	require.Equal(t, testlib.Originals(t)["a.bin"], testlib.SHA256(t, filepath.Join(dir, "a.bin")))
	_, err := os.Stat(filepath.Join(dir, "a.bin.1"))
	require.NoError(t, err, "without purge the damaged original is kept as a.bin.1")
}

func TestShimCancelledBeforeRunReturnsCancelled(t *testing.T) {
	testlib.RequireLib(t)
	dir := testlib.CopyCase(t, "repairable")
	job := bindings.New(filepath.Join(dir, "set.par2"), dir, 64<<20, 1, 2, 0)
	require.NotZero(t, job)
	defer bindings.Free(job)
	bindings.Cancel(job)
	require.EqualValues(t, bindings.Cancelled, runJob(t, job, 1))
}

// runJob starts job and polls until its thread has finished.
func runJob(t *testing.T, job uintptr, repair int32) int32 {
	t.Helper()
	require.Zero(t, bindings.Start(job, repair))
	var p bindings.Progress
	for bindings.ReadProgress(job, &p); p.Done == 0; bindings.ReadProgress(job, &p) {
		time.Sleep(time.Millisecond)
	}
	return int32(p.Result)
}
