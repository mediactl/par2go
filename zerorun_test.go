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
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mediactl/par2go/internal/testlib"
)

// An extra file holding a long run of zeros is scanned about as fast as
// one that holds none. The set's file has whole blocks of zeros, which a
// verify finds there; unpatched, every byte offset of the extra's run then
// matched one of them again, and par2 hashed the whole block to learn it
// was a duplicate, at each offset: 1.7 million 1 MiB blocks for the gap a
// Matroska file opens with, half an hour a file on a set of 7 GB episodes
// (2026-11-27). The 4 MiB run here took the unpatched library seconds
// (shim/patches/0003-duplicate-run.patch).
func TestALongRunOfZerosInAnExtraFileIsNotRehashedAtEveryOffset(t *testing.T) {
	testlib.RequireLib(t)
	const size = 4 << 20
	verify := func(name string, fill func(b []byte)) (Result, time.Duration) {
		dir := testlib.CopyCase(t, "zerorun")
		b := make([]byte, size+8192)
		r := rand.New(rand.NewPCG(7, 11))
		for i := range b {
			b[i] = byte(r.Uint32())
		}
		fill(b)
		extra := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(extra, b, 0o644))
		start := time.Now()
		res, err := Verify(context.Background(), index(dir), Options{ExtraFiles: []string{extra}})
		require.NoError(t, err, res.Log)
		return res, time.Since(start)
	}

	plain, base := verify("plain.bin", func([]byte) {})
	zeros, took := verify("zeros.bin", func(b []byte) { clear(b[4096 : 4096+size]) })

	require.Equal(t, RepairPossible, plain.Status, plain.Log)
	require.Equal(t, plain.Status, zeros.Status, zeros.Log)
	require.Equal(t, plain.BlocksNeeded, zeros.BlocksNeeded)
	require.Equal(t, plain.Headers.DamagedFiles, zeros.Headers.DamagedFiles)
	require.Less(t, took, 10*base+500*time.Millisecond,
		"the run of zeros cost %s against %s for a file without one", took, base)
}
