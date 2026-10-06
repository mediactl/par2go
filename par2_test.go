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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mediactl/par2go/internal/testlib"
)

func index(dir string) string { return filepath.Join(dir, "set.par2") }

func TestVerifyReportsEachCase(t *testing.T) {
	testlib.RequireLib(t)
	for name, want := range map[string]struct {
		status Status
		needed int
		extras []string
	}{
		"intact":       {AllCorrect, 0, nil},
		"repairable":   {RepairPossible, 0, nil},
		"unrepairable": {RepairNotPossible, 3, nil},
		"renamed":      {RepairPossible, 0, []string{"z75QO.part070.rar"}},
		"volnames":     {RepairPossible, 0, []string{"set.vol-01.par2"}},
	} {
		t.Run(name, func(t *testing.T) {
			dir := testlib.CopyCase(t, name)
			var extras []string
			for _, e := range want.extras {
				extras = append(extras, filepath.Join(dir, e))
			}
			before := testlib.SHA256(t, index(dir))
			res, err := Verify(context.Background(), index(dir), Options{ExtraFiles: extras, Purge: true})
			require.NoError(t, err, res.Log)
			require.Equal(t, want.status, res.Status, res.Log)
			require.Equal(t, want.needed, res.BlocksNeeded)
			require.Equal(t, 13, res.Headers.DataBlocks)
			require.Equal(t, before, testlib.SHA256(t, index(dir)), "Verify ignores Purge")
		})
	}
}

func TestVerifyNamesWhatItFound(t *testing.T) {
	testlib.RequireLib(t)
	dir := testlib.CopyCase(t, "renamed")
	res, err := Verify(context.Background(), index(dir), Options{
		ExtraFiles: []string{filepath.Join(dir, "z75QO.part070.rar")},
	})
	require.NoError(t, err)
	states := map[string]FileState{}
	for _, f := range res.Files {
		states[f.Name] = f.State
	}
	require.Equal(t, map[string]FileState{"a.bin": Renamed, "b.bin": Complete}, states)
}

func TestRepairRestoresTheOriginals(t *testing.T) {
	testlib.RequireLib(t)
	for name, extras := range map[string][]string{
		"repairable": nil,
		"renamed":    {"z75QO.part070.rar"},
		"volnames":   {"set.vol-01.par2"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := testlib.CopyCase(t, name)
			var abs []string
			for _, e := range extras {
				abs = append(abs, filepath.Join(dir, e))
			}
			res, err := Repair(context.Background(), index(dir), Options{ExtraFiles: abs})
			require.NoError(t, err, res.Log)
			require.Equal(t, Repaired, res.Status, res.Log)
			for f, sum := range testlib.Originals(t) {
				require.Equal(t, sum, testlib.SHA256(t, filepath.Join(dir, f)), f)
			}
		})
	}
}

func TestRepairOfAnUnrepairableSetChangesNothing(t *testing.T) {
	testlib.RequireLib(t)
	dir := testlib.CopyCase(t, "unrepairable")
	before := testlib.SHA256(t, filepath.Join(dir, "a.bin"))
	res, err := Repair(context.Background(), index(dir), Options{})
	require.NoError(t, err)
	require.Equal(t, RepairNotPossible, res.Status)
	require.Equal(t, 3, res.BlocksNeeded)
	require.Equal(t, before, testlib.SHA256(t, filepath.Join(dir, "a.bin")))
}

func TestPurgeRemovesBackupsAndParFilesAfterARepair(t *testing.T) {
	testlib.RequireLib(t)
	dir := testlib.CopyCase(t, "repairable")
	res, err := Repair(context.Background(), index(dir), Options{Purge: true})
	require.NoError(t, err, res.Log)
	require.Equal(t, Repaired, res.Status)
	left, _ := filepath.Glob(filepath.Join(dir, "*"))
	var names []string
	for _, p := range left {
		names = append(names, filepath.Base(p))
	}
	require.ElementsMatch(t, []string{"a.bin", "b.bin"}, names)
}

func TestPurgeOnAnIntactSetRemovesTheParFiles(t *testing.T) {
	testlib.RequireLib(t)
	dir := testlib.CopyCase(t, "intact")
	res, err := Repair(context.Background(), index(dir), Options{Purge: true})
	require.NoError(t, err, res.Log)
	require.Equal(t, AllCorrect, res.Status)
	_, err = os.Stat(index(dir))
	require.True(t, os.IsNotExist(err))
}

// Review Focus 1.
func TestPathsWithSpacesAndNonASCII(t *testing.T) {
	testlib.RequireLib(t)
	src := testlib.CopyCase(t, "repairable")
	dir := filepath.Join(t.TempDir(), "my set", "ünïcødé")
	require.NoError(t, os.MkdirAll(filepath.Dir(dir), 0o755))
	require.NoError(t, os.Rename(src, dir))
	res, err := Repair(context.Background(), index(dir), Options{})
	require.NoError(t, err, res.Log)
	require.Equal(t, Repaired, res.Status)
	require.Equal(t, testlib.Originals(t)["a.bin"], testlib.SHA256(t, filepath.Join(dir, "a.bin")))
}

// Review Focus 2.
func TestAGarbageIndexIsAnErrorNotACrash(t *testing.T) {
	testlib.RequireLib(t)
	for name, body := range map[string][]byte{
		"empty":   nil,
		"garbage": []byte("this is not a par2 file, not even close\n"),
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(index(dir), body, 0o644))
			res, err := Verify(context.Background(), index(dir), Options{})
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrInsufficientCriticalData) || errors.Is(err, ErrLogic), "%v", err)
			require.NotEmpty(t, res.Log)
		})
	}
}
