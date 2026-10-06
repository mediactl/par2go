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
	"debug/elf"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mediactl/par2go/internal/testlib"
)

// consumer includes par2's headers before anything else, so the published
// headers must stand on their own (patch 0003), and calls Process from the
// set's directory, since Process ignores its basepath argument.
const consumer = `#include <par2/libpar2.h>
#include <par2/par2repairer.h>
#include <iostream>
#include <sstream>

int main(int argc, char **argv) {
  if (argc != 2) return 100;
  std::ostringstream out;
  Par2::Par2Repairer r(out, out, Par2::nlQuiet);
  std::vector<std::string> extra;
  Par2::Result res = r.Process(64 << 20, "./", 1, 2, argv[1], extra,
                               false, false, false, false, 0);
  std::cerr << out.str();
  return static_cast<int>(res);
}
`

// The published libpar2.so builds and runs a C++ program -- compiled
// against the patched headers and flags shim/build.sh stages in
// libpar2-dev -- and carries par2go's patches.
func TestThePublishedLibpar2IsUsableFromCxx(t *testing.T) {
	dir := os.Getenv("PAR2GO_LIBPAR2_DIR")
	if dir == "" {
		if os.Getenv("PAR2GO_REQUIRE") == "1" {
			t.Fatal("PAR2GO_REQUIRE=1 but PAR2GO_LIBPAR2_DIR is not set")
		}
		t.Skip("PAR2GO_LIBPAR2_DIR not set (shim/out: libpar2.so and libpar2-dev)")
	}
	cxx, err := exec.LookPath("c++")
	require.NoError(t, err, "a C++ compiler is needed to use libpar2.so")

	so := filepath.Join(dir, "libpar2.so")
	f, err := elf.Open(so)
	require.NoError(t, err)
	needed, _ := f.ImportedLibraries()
	f.Close()
	require.Contains(t, needed, "libstdc++.so.6", "libpar2.so must share libstdc++ with its user")

	root := filepath.Join(dir, "libpar2-dev")
	flags, err := os.ReadFile(filepath.Join(root, "par2-flags.txt"))
	require.NoError(t, err)
	require.Contains(t, string(flags), "-fno-rtti")

	work := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(work, "main.cpp"), []byte(consumer), 0o644))
	args := append([]string{"-std=c++20"}, strings.Fields(string(flags))...)
	args = append(args, "-I", filepath.Join(root, "include"), "-I", root,
		filepath.Join(work, "main.cpp"), "-L", dir, "-lpar2", "-Wl,-rpath,"+dir,
		"-o", filepath.Join(work, "verify"))
	out, err := exec.Command(cxx, args...).CombinedOutput()
	require.NoError(t, err, string(out))

	for name, want := range map[string]int{
		"intact":  0, // eSuccess
		"unicode": 1, // eRepairPossible: the UTF-8 name was found (patch 0001)
	} {
		set := testlib.CopyCase(t, name)
		cmd := exec.Command(filepath.Join(work, "verify"), "set.par2")
		cmd.Dir = set
		out, err := cmd.CombinedOutput()
		code := 0
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			require.NoError(t, err)
		}
		require.Equal(t, want, code, "%s: %s", name, out)
	}
}
