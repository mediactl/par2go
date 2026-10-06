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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mediactl/par2go/internal/bindings"
	"github.com/mediactl/par2go/internal/testlib"
)

// p2_start runs the job on the shim's own thread and returns at once: no
// Go thread waits inside C for the length of a job.
func TestStartReturnsWhileTheJobRuns(t *testing.T) {
	testlib.RequireLib(t)
	dir := bigSet(t)
	job := bindings.New(index(dir), dir, 64<<20, 1, 1, 0)
	require.NotZero(t, job)
	defer bindings.Free(job)

	begin := time.Now()
	require.Zero(t, bindings.Start(job, 0))
	require.Less(t, time.Since(begin), 50*time.Millisecond, "p2_start blocked")
	var p bindings.Progress
	bindings.ReadProgress(job, &p)
	require.Zero(t, p.Done, "a 128 MiB verify finished inside p2_start")
	require.EqualValues(t, 1, bindings.ActiveJobs())

	for p.Done == 0 {
		time.Sleep(5 * time.Millisecond)
		bindings.ReadProgress(job, &p)
	}
	require.EqualValues(t, bindings.RepairPossible, p.Result)
	require.NotZero(t, bindings.Start(job, 0), "a job starts once")
}

// p2_free on a running job cancels it and joins its thread before
// releasing anything, so freeing is safe whatever Go is doing.
func TestFreeingARunningJobCancelsAndJoinsIt(t *testing.T) {
	testlib.RequireLib(t)
	dir := bigSet(t)
	before := testlib.SHA256(t, filepath.Join(dir, "big.bin"))
	job := bindings.New(index(dir), dir, 64<<20, 1, 1, 0)
	require.NotZero(t, job)
	require.Zero(t, bindings.Start(job, 1))
	time.Sleep(20 * time.Millisecond)
	bindings.Free(job)

	require.Zero(t, bindings.ActiveJobs())
	require.Equal(t, before, testlib.SHA256(t, filepath.Join(dir, "big.bin")))
	_, err := os.Stat(filepath.Join(dir, "big.bin.1"))
	require.True(t, os.IsNotExist(err))
}
