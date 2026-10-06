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
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mediactl/par2go/internal/bindings"
)

// jobMu serialises jobs: Process reads process-wide statics.
var jobMu sync.Mutex

// inFlight counts p2_run calls that have not returned. It is zero whenever
// Verify or Repair returns, panics or Goexits: a job is never freed under C.
var inFlight atomic.Int32

// testHook, when set by a test, is called with true just after jobMu is
// taken and with false just before it is released.
var testHook func(entered bool)

// Available loads libpar2go.so once and reports whether it can be used.
func Available() error { return bindings.Load() }

// Verify checks the set named by index without changing anything.
func Verify(ctx context.Context, index string, opts Options) (Result, error) {
	return run(ctx, index, opts, false)
}

// Repair verifies the set and, when the damage is repairable, repairs it.
func Repair(ctx context.Context, index string, opts Options) (Result, error) {
	return run(ctx, index, opts, true)
}

func run(ctx context.Context, index string, opts Options, repair bool) (Result, error) {
	if err := Available(); err != nil {
		return Result{}, err
	}
	r, err := opts.resolve(index)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	jobMu.Lock()
	defer jobMu.Unlock()
	if testHook != nil {
		testHook(true)
		defer testHook(false)
	}
	if err := ctx.Err(); err != nil { // cancelled while waiting for the lock
		return Result{}, err
	}

	job := bindings.New(r.index, r.dir, r.memoryLimit, int32(r.threads), int32(r.fileThreads),
		b2i(repair && r.purge))
	if job == 0 {
		return Result{}, fmt.Errorf("%w: p2_new failed", ErrMemory)
	}
	for _, e := range r.extras {
		if bindings.AddExtra(job, e) != 0 {
			bindings.Free(job)
			return Result{}, fmt.Errorf("%w: p2_add_extra failed", ErrMemory)
		}
	}

	done := make(chan int32, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		inFlight.Add(1)
		code := bindings.Run(job, b2i(repair))
		inFlight.Add(-1)
		done <- code
	}()
	// However this function is left -- a return, a panic in Progress, or a
	// runtime.Goexit there (t.FailNow) -- the job is freed only after
	// p2_run has returned. The deferred Unlock runs after this.
	finished := false
	defer func() {
		if !finished {
			bindings.Cancel(job)
			<-done
		}
		bindings.Free(job)
	}()
	code := wait(ctx, job, done, r.pollEvery, opts.Progress)
	finished = true
	return finish(ctx, job, code)
}

// wait returns p2_run's result once it has returned. A panic or Goexit in
// progress leaves it early; run's deferred guard then cancels and waits.
func wait(ctx context.Context, job uintptr, done <-chan int32, every time.Duration, progress func(Progress)) int32 {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if progress != nil {
				progress(readProgress(job))
			}
		case <-ctx.Done():
			bindings.Cancel(job)
			return <-done
		case code := <-done:
			return code
		}
	}
}

func readProgress(job uintptr) Progress {
	var p bindings.Progress
	bindings.ReadProgress(job, &p)
	return Progress{Phase: Phase(p.Phase), File: bindings.CString(p.File[:]), PerMille: int(p.PerMille)}
}

func finish(ctx context.Context, job uintptr, code int32) (Result, error) {
	var c bindings.Counts
	bindings.ReadCounts(job, &c)
	res := Result{
		Headers: Headers{
			SetID:            bindings.CString(c.SetID[:]),
			BlockSize:        c.BlockSize,
			DataBlocks:       int(c.DataBlocks),
			RecoveryBlocks:   int(c.RecoveryBlocks),
			RecoverableFiles: int(c.RecoverableFiles),
			OtherFiles:       int(c.OtherFiles),
			CompleteFiles:    int(c.CompleteFiles),
			RenamedFiles:     int(c.RenamedFiles),
			DamagedFiles:     int(c.DamagedFiles),
			MissingFiles:     int(c.MissingFiles),
		},
		Log: readLog(job),
	}
	for i := int32(0); i < int32(c.Files); i++ {
		var f bindings.File
		if bindings.ReadFile(job, i, &f) != 0 {
			break
		}
		res.Files = append(res.Files, File{
			Name:            bindings.CString(f.Name[:]),
			State:           FileState(f.State),
			BlocksAvailable: int(f.BlocksAvailable),
			BlocksTotal:     int(f.BlocksTotal),
		})
	}
	switch code {
	case bindings.Success:
		res.Status = AllCorrect
		if c.RepairAttempted == 1 {
			res.Status = Repaired
		}
		return res, nil
	case bindings.RepairPossible:
		res.Status = RepairPossible
		return res, nil
	case bindings.RepairNotPossible:
		res.Status = RepairNotPossible
		res.BlocksNeeded = max(0, int(c.MissingBlocks-c.RecoveryBlocks))
		return res, nil
	case bindings.Cancelled:
		if err := ctx.Err(); err != nil {
			return res, err
		}
		return res, context.Canceled
	case bindings.InvalidArgs, bindings.Invalid:
		return res, fmt.Errorf("%w (par2 result %d)", ErrInvalidOptions, code)
	case bindings.InsufficientCriticalData:
		return res, ErrInsufficientCriticalData
	case bindings.RepairFailed:
		return res, ErrRepairFailed
	case bindings.FileIOError:
		return res, ErrIO
	case bindings.MemoryError:
		return res, ErrMemory
	case bindings.LogicError:
		return res, ErrLogic
	}
	return res, fmt.Errorf("%w: unknown par2 result %d", ErrLogic, code)
}

func readLog(job uintptr) string {
	buf := make([]byte, 4096)
	n := bindings.ReadLog(job, &buf[0], uint64(len(buf)))
	return string(buf[:min(n, uint64(len(buf)))])
}

func b2i(b bool) int32 {
	if b {
		return 1
	}
	return 0
}
