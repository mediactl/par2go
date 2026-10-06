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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mediactl/par2go/internal/testlib"
)

const unicodeName = "Amélie café – 2001.bin"

func unicodeOriginal(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testlib.Root(), "testdata", "sha256-unicode.txt"))
	require.NoError(t, err)
	return strings.Fields(string(b))[0]
}

// Names stored as UTF-8 (par2cmdline, ParPar, MultiPar all write UTF-8)
// must come back unchanged: not re-encoded as if they were Latin-1.
func TestUTF8FileNamesRoundTrip(t *testing.T) {
	testlib.RequireLib(t)
	dir := testlib.CopyCase(t, "unicode")
	res, err := Verify(context.Background(), index(dir), Options{})
	require.NoError(t, err, res.Log)
	require.Equal(t, RepairPossible, res.Status, res.Log)
	require.Len(t, res.Files, 1)
	require.Equal(t, unicodeName, res.Files[0].Name)
	require.Equal(t, Damaged, res.Files[0].State)

	res, err = Repair(context.Background(), index(dir), Options{})
	require.NoError(t, err, res.Log)
	require.Equal(t, Repaired, res.Status, res.Log)
	require.Equal(t, unicodeOriginal(t), testlib.SHA256(t, filepath.Join(dir, unicodeName)))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		n := e.Name()
		require.True(t, n == unicodeName || n == unicodeName+".1" || strings.HasPrefix(n, "set."),
			"repair wrote an unexpected file %q", n)
	}
}
