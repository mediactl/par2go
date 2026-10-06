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
	"sync"
	"time"

	"github.com/mediactl/par2go/internal/bindings"
)

// jobMu serialises jobs: Process reads process-wide statics.
var jobMu sync.Mutex

// doneEvery is how often a running job is checked for completion: a
// load of one atomic in the shim, so cheap enough to keep latency low.
const doneEvery = 5 * time.Millisecond

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

	// The job runs on the shim's own thread; no Go thread waits in C for
	// it. p2_free cancels and joins a job still running, so freeing is
	// safe however this function is left -- a return, a panic in
	// Progress, or a runtime.Goexit there (t.FailNow). The deferred
	// Unlock runs after it.
	if code := bindings.Start(job, b2i(repair)); code != 0 {
		bindings.Free(job)
		return Result{}, fmt.Errorf("%w: p2_start returned %d", ErrLogic, code)
	}
	defer bindings.Free(job)
	code := wait(ctx, job, r.pollEvery, opts.Progress)
	return finish(ctx, job, code)
}

// wait polls job until its thread has finished and returns the result,
// calling progress every `every` and cancelling the job once ctx is done.
func wait(ctx context.Context, job uintptr, every time.Duration, progress func(Progress)) int32 {
	tick := time.NewTicker(doneEvery)
	defer tick.Stop()
	ctxDone := ctx.Done()
	next := time.Now()
	for {
		var p bindings.Progress
		bindings.ReadProgress(job, &p)
		if p.Done != 0 {
			return int32(p.Result)
		}
		if progress != nil && !time.Now().Before(next) {
			progress(toProgress(p))
			next = time.Now().Add(every)
		}
		select {
		case <-ctxDone:
			bindings.Cancel(job)
			ctxDone = nil // keep polling until the job has stopped
		case <-tick.C:
		}
	}
}

func toProgress(p bindings.Progress) Progress {
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
