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

// ABIVersion must equal the shim's P2_ABI_VERSION.
const ABIVersion = 1

// NameMax is P2_NAME_MAX.
const NameMax = 4096

// p2_run results; 0-8 are par2cmdline-turbo's Par2::Result.
const (
	Success                  = 0
	RepairPossible           = 1
	RepairNotPossible        = 2
	InvalidArgs              = 3
	InsufficientCriticalData = 4
	RepairFailed             = 5
	FileIOError              = 6
	LogicError               = 7
	MemoryError              = 8
	Cancelled                = 100
	Invalid                  = 101
)

const (
	PhaseVerifying = 1
	PhaseRepairing = 2
)

const (
	FileComplete = 1
	FileRenamed  = 2
	FileDamaged  = 3
	FileMissing  = 4
)

const (
	sizeofProgress = 1
	sizeofCounts   = 2
	sizeofFile     = 3
)

// Progress mirrors p2_progress.
type Progress struct {
	Phase         int64
	PerMille      int64
	FileTruncated int64
	File          [NameMax]byte
}

// Counts mirrors p2_counts.
type Counts struct {
	RepairAttempted  int64
	BlockSize        int64
	DataBlocks       int64
	RecoveryBlocks   int64
	RecoverableFiles int64
	OtherFiles       int64
	CompleteFiles    int64
	RenamedFiles     int64
	DamagedFiles     int64
	MissingFiles     int64
	AvailableBlocks  int64
	MissingBlocks    int64
	Files            int64
	SetID            [40]byte
}

// File mirrors p2_file_result.
type File struct {
	State           int64
	BlocksAvailable int64
	BlocksTotal     int64
	NameTruncated   int64
	Name            [NameMax]byte
}

// CString returns b up to its first NUL.
func CString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
