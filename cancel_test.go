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
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mediactl/par2go/internal/testlib"
)

// bigSet makes a 128 MiB set in 1 MiB blocks with 20 recovery blocks and
// damages 16 blocks, so verify and repair each take long enough to cancel.
func bigSet(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("large set; not in -short")
	}
	par2, err := exec.LookPath("par2")
	if err != nil {
		if os.Getenv("PAR2GO_REQUIRE") == "1" {
			t.Fatal("PAR2GO_REQUIRE=1 but par2 is not on PATH")
		}
		t.Skip("par2 not on PATH")
	}
	dir := t.TempDir()
	gen := func(args ...string) {
		cmd := exec.Command("go", append([]string{"run", "./internal/testgen"}, args...)...)
		cmd.Dir = testlib.Root()
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	gen("data", "-seed", "7", "-size", "134217728", "-o", filepath.Join(dir, "big.bin"))
	cmd := exec.Command(par2, "create", "-q", "-q", "-s1048576", "-c20", "-n1", "set.par2", "big.bin")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	gen("damage", "-o", filepath.Join(dir, "big.bin"), "-bs", "1048576", "-first", "10", "-count", "16")
	return dir
}

func TestCancelDuringRepairLeavesNoPartialTarget(t *testing.T) {
	testlib.RequireLib(t)
	dir := bigSet(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sawRepair := false
	res, err := Repair(ctx, index(dir), Options{
		Threads: 1, MemoryLimit: 8 << 20, PollEvery: 5 * time.Millisecond,
		Progress: func(p Progress) {
			if p.Phase == Repairing {
				sawRepair = true
				cancel()
			}
		},
	})
	require.True(t, sawRepair, "the repair finished before a poll saw it; make bigSet larger")
	require.ErrorIs(t, err, context.Canceled, res.Log)

	// The damaged original is back under its own name, no backup is left
	// beside it, and the set is exactly as repairable as before: a plain
	// Repair, with no extra files, finishes the job.
	_, err = os.Stat(filepath.Join(dir, "big.bin"))
	require.NoError(t, err, "the cancelled repair left no big.bin")
	_, err = os.Stat(filepath.Join(dir, "big.bin.1"))
	require.True(t, os.IsNotExist(err), "the damaged original was left as big.bin.1")
	res, err = Repair(context.Background(), index(dir), Options{})
	require.NoError(t, err, res.Log)
	require.Equal(t, Repaired, res.Status, res.Log)
	res, err = Verify(context.Background(), index(dir), Options{})
	require.NoError(t, err)
	require.Equal(t, AllCorrect, res.Status)
}

// Review Focus 4.
func TestADeadlineDuringVerifyChangesNothing(t *testing.T) {
	testlib.RequireLib(t)
	dir := bigSet(t)
	before := testlib.SHA256(t, filepath.Join(dir, "big.bin"))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := Repair(ctx, index(dir), Options{Threads: 1, FileThreads: 1})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, before, testlib.SHA256(t, filepath.Join(dir, "big.bin")))
	_, err = os.Stat(filepath.Join(dir, "big.bin.1"))
	require.True(t, os.IsNotExist(err), "verify must not have started a repair")
}

// Review Focus 3.
func TestAPanickingProgressCallbackReleasesTheJob(t *testing.T) {
	testlib.RequireLib(t)
	dir := testlib.CopyCase(t, "repairable")
	require.PanicsWithValue(t, "boom", func() {
		_, _ = Repair(context.Background(), index(dir), Options{
			PollEvery: time.Microsecond,
			Progress:  func(Progress) { panic("boom") },
		})
	})
	// p2_run had returned before the job was freed and the panic resumed.
	require.Zero(t, inFlight.Load(), "Repair panicked while p2_run was still running")
	// The mutex was released and the job freed: the next run works.
	dir2 := testlib.CopyCase(t, "repairable")
	res, err := Repair(context.Background(), index(dir2), Options{})
	require.NoError(t, err)
	require.Equal(t, Repaired, res.Status)
}

// Review Focus 5.
func TestAContextCancelledBeforeTheCallNeverEntersTheLibrary(t *testing.T) {
	testlib.RequireLib(t)
	dir := testlib.CopyCase(t, "repairable")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	entered := false
	testHook = func(bool) { entered = true }
	defer func() { testHook = nil }()
	_, err := Repair(ctx, index(dir), Options{})
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, entered)
}

func TestJobsRunOneAtATime(t *testing.T) {
	testlib.RequireLib(t)
	var inside, maxInside atomic.Int32
	testHook = func(entered bool) {
		if entered {
			n := inside.Add(1)
			for {
				m := maxInside.Load()
				if n <= m || maxInside.CompareAndSwap(m, n) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond) // widen the window a race would need
		} else {
			inside.Add(-1)
		}
	}
	defer func() { testHook = nil }()

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range 2 {
		dir := testlib.CopyCase(t, "repairable")
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := Repair(context.Background(), index(dir), Options{})
			if err == nil && res.Status != Repaired {
				err = errors.New(res.Status.String())
			}
			errs[i] = err
		}()
	}
	wg.Wait()
	require.NoError(t, errors.Join(errs...))
	require.EqualValues(t, 1, maxInside.Load())
}

// Review Focus 5, second half: cancelled while queued behind another job.
func TestAContextCancelledWhileWaitingForTheLockNeverRuns(t *testing.T) {
	testlib.RequireLib(t)
	release := make(chan struct{})
	holding := make(chan struct{})
	jobMu.Lock()
	go func() { close(holding); <-release; jobMu.Unlock() }()
	<-holding

	ctx, cancel := context.WithCancel(context.Background())
	entered := false
	testHook = func(e bool) {
		if e {
			entered = true
		}
	}
	defer func() { testHook = nil }()
	errc := make(chan error, 1)
	dir := testlib.CopyCase(t, "repairable")
	go func() { _, err := Repair(ctx, index(dir), Options{}); errc <- err }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	close(release)
	require.ErrorIs(t, <-errc, context.Canceled)
	// The hook runs as soon as the lock is taken, before the ctx re-check:
	// it got the lock and still never ran, so a.bin is still damaged.
	require.True(t, entered, "it took the lock")
	require.NotEqual(t, testlib.Originals(t)["a.bin"], testlib.SHA256(t, filepath.Join(dir, "a.bin")))
}

// A Progress callback that ends its goroutine without panicking (as
// t.FailNow and require.* do, through runtime.Goexit) must still leave
// the job cancelled and finished before it is freed.
func TestAProgressCallbackThatExitsTheGoroutineReleasesTheJob(t *testing.T) {
	testlib.RequireLib(t)
	dir := testlib.CopyCase(t, "repairable")
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		_, _ = Repair(context.Background(), index(dir), Options{
			PollEvery: time.Microsecond,
			Progress:  func(Progress) { runtime.Goexit() },
		})
	}()
	<-exited
	require.Zero(t, inFlight.Load(), "Repair's goroutine exited while p2_run was still running")
	dir2 := testlib.CopyCase(t, "repairable")
	res, err := Repair(context.Background(), index(dir2), Options{})
	require.NoError(t, err)
	require.Equal(t, Repaired, res.Status)
}
