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
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mediactl/par2go/internal/testlib"
)

var needRE = regexp.MustCompile(`You need (\d+) more recovery blocks`)

// cli runs par2 and returns its exit code and the blocks it says it needs.
func cli(t *testing.T, op, dir string, extras []string) (int, int) {
	t.Helper()
	args := append([]string{op, "set.par2"}, extras...)
	cmd := exec.Command("par2", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else {
		require.NoError(t, err)
	}
	need := 0
	if m := needRE.FindSubmatch(out); m != nil {
		need, _ = strconv.Atoi(string(m[1]))
	}
	return code, need
}

// The CLI's exit code is par2's Result; par2go's verdict must match it on
// a fresh copy of every fixture. This is what proves the members the shim
// reads mean what par2's text says.
func TestVerdictsMatchTheCLI(t *testing.T) {
	testlib.RequireLib(t)
	if _, err := exec.LookPath("par2"); err != nil {
		if os.Getenv("PAR2GO_REQUIRE") == "1" {
			t.Fatal("PAR2GO_REQUIRE=1 but par2 is not on PATH")
		}
		t.Skip("par2 not on PATH")
	}
	statusOfCode := map[int]Status{0: AllCorrect, 1: RepairPossible, 2: RepairNotPossible}
	extras := map[string][]string{"renamed": {"z75QO.part070.rar"}, "volnames": {"set.vol-01.par2"}}
	for _, name := range []string{"intact", "repairable", "unrepairable", "renamed", "volnames", "unicode"} {
		t.Run(name, func(t *testing.T) {
			cliDir, goDir := testlib.CopyCase(t, name), testlib.CopyCase(t, name)
			code, need := cli(t, "v", cliDir, extras[name])
			var abs []string
			for _, e := range extras[name] {
				abs = append(abs, filepath.Join(goDir, e))
			}
			res, err := Verify(context.Background(), index(goDir), Options{ExtraFiles: abs})
			require.NoError(t, err)
			require.Equal(t, statusOfCode[code], res.Status, "par2 v exited %d", code)
			require.Equal(t, need, res.BlocksNeeded)

			rcode, _ := cli(t, "r", cliDir, extras[name])
			rres, rerr := Repair(context.Background(), index(goDir), Options{ExtraFiles: abs})
			require.NoError(t, rerr)
			require.Equal(t, rcode == 0, rres.Status == AllCorrect || rres.Status == Repaired,
				"par2 r exited %d, par2go said %s", rcode, rres.Status)
		})
	}
}
