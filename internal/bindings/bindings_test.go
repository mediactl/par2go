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

package bindings_test

import (
	"debug/elf"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mediactl/par2go/internal/bindings"
	"github.com/mediactl/par2go/internal/testlib"
)

func TestCandidatesTryTheEnvironmentFirst(t *testing.T) {
	env := func(v string) func(string) string {
		return func(string) string { return v }
	}
	require.Equal(t, []string{"/x/libpar2shim.so", "libpar2shim.so"}, bindings.Candidates(env("/x/libpar2shim.so")))
	require.Equal(t, []string{"libpar2shim.so"}, bindings.Candidates(env("")))
}

func TestTheLibraryLoadsWithAMatchingABI(t *testing.T) {
	testlib.RequireLib(t)
	require.EqualValues(t, bindings.ABIVersion, bindings.ABIVer())
	require.NotEmpty(t, bindings.Path())
}

func TestTheLibraryNeedsNothingButGlibc(t *testing.T) {
	testlib.RequireLib(t)
	f, err := elf.Open(bindings.Path())
	require.NoError(t, err)
	defer f.Close()

	needed, err := f.ImportedLibraries()
	require.NoError(t, err)
	allowed := map[string]bool{
		"libc.so.6": true, "libm.so.6": true,
		"ld-linux-x86-64.so.2": true, "ld-linux-aarch64.so.1": true,
	}
	for _, n := range needed {
		require.True(t, allowed[n], "DT_NEEDED %s: only glibc may be dynamic (is -static-libstdc++ missing?)", n)
	}

	syms, err := f.DynamicSymbols()
	require.NoError(t, err)
	for _, s := range syms {
		defined := s.Section != elf.SHN_UNDEF
		global := elf.ST_BIND(s.Info) == elf.STB_GLOBAL || elf.ST_BIND(s.Info) == elf.STB_WEAK
		if defined && global && s.Name != "" {
			require.True(t, strings.HasPrefix(s.Name, "p2_"), "exported symbol %s: only p2_* may be exported", s.Name)
		}
	}
}
