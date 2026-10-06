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

	"github.com/mediactl/par2go/internal/bindings"
)

// Status is what a run concluded about the set.
type Status int

const (
	// AllCorrect: every file verified; nothing was changed.
	AllCorrect Status = iota + 1
	// Repaired: Repair rebuilt or renamed files and they now verify.
	Repaired
	// RepairPossible: Verify found damage the recovery data can repair.
	RepairPossible
	// RepairNotPossible: too few recovery blocks; see Result.BlocksNeeded.
	RepairNotPossible
)

func (s Status) String() string {
	switch s {
	case AllCorrect:
		return "AllCorrect"
	case Repaired:
		return "Repaired"
	case RepairPossible:
		return "RepairPossible"
	case RepairNotPossible:
		return "RepairNotPossible"
	}
	return "Unknown"
}

// FileState is what verification found for one file of the set.
type FileState int

const (
	Complete FileState = iota + 1
	Renamed            // found intact under another name
	Damaged            // present with missing or corrupt blocks
	Missing
)

// Phase is the stage a running job is in.
type Phase int

const (
	// Loading reads the par2 files; PerMille is of the current one.
	Loading Phase = iota + 1
	// Verifying scans the data files; PerMille is of all of them.
	Verifying
	// Repairing rebuilds missing blocks; PerMille is of the whole repair.
	Repairing
)

func (p Phase) String() string {
	switch p {
	case Loading:
		return "Loading"
	case Verifying:
		return "Verifying"
	case Repairing:
		return "Repairing"
	}
	return "Unknown"
}

// Progress is one poll of a running job.
type Progress struct {
	Phase    Phase
	File     string
	PerMille int
}

// Headers describe the set. The file counts are what verification found,
// before any repair.
type Headers struct {
	SetID            string
	BlockSize        int64
	DataBlocks       int
	RecoveryBlocks   int
	RecoverableFiles int
	OtherFiles       int
	CompleteFiles    int
	RenamedFiles     int
	DamagedFiles     int
	MissingFiles     int
}

// File is one file of the set as verification found it. Name is relative
// to Options.Dir; it is empty for a file the set lists without a
// description packet.
type File struct {
	Name            string
	State           FileState
	BlocksAvailable int
	BlocksTotal     int
}

// Result is what one Verify or Repair concluded.
type Result struct {
	Status Status
	// BlocksNeeded is how many more recovery blocks would make repair
	// possible; non-zero only for RepairNotPossible.
	BlocksNeeded int
	Headers      Headers
	Files        []File
	// Log is the last 4 KiB of par2's own output.
	Log string
}

var (
	ErrUnavailable              = bindings.ErrUnavailable
	ErrInvalidOptions           = errors.New("par2go: invalid options")
	ErrInsufficientCriticalData = errors.New("par2go: the par2 files lack critical data")
	ErrIO                       = errors.New("par2go: file I/O error")
	ErrMemory                   = errors.New("par2go: out of memory")
	ErrRepairFailed             = errors.New("par2go: repair completed but files still do not verify")
	ErrLogic                    = errors.New("par2go: internal library error")
)
