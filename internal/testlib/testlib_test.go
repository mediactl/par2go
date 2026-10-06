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

package testlib_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mediactl/par2go/internal/testlib"
)

func TestTheIntactFixtureMatchesTheRecordedOriginals(t *testing.T) {
	dir := testlib.CopyCase(t, "intact")
	for name, sum := range testlib.Originals(t) {
		require.Equal(t, sum, testlib.SHA256(t, filepath.Join(dir, name)), name)
	}
}

func TestCopyCaseIsACopy(t *testing.T) {
	dir := testlib.CopyCase(t, "intact")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.bin"), []byte("x"), 0o644))
	again := testlib.CopyCase(t, "intact")
	require.Equal(t, testlib.Originals(t)["a.bin"], testlib.SHA256(t, filepath.Join(again, "a.bin")))
}
