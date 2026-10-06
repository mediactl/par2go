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

// Package testlib holds helpers the module's tests share.
package testlib

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mediactl/par2go/internal/bindings"
)

// RequireLib skips t when libpar2go.so cannot be loaded, unless
// PAR2GO_REQUIRE=1, which turns the skip into a failure (CI sets it).
func RequireLib(t testing.TB) {
	t.Helper()
	if err := bindings.Load(); err != nil {
		if os.Getenv("PAR2GO_REQUIRE") == "1" {
			t.Fatalf("PAR2GO_REQUIRE=1 but the library did not load: %v", err)
		}
		t.Skipf("libpar2go.so unavailable (set PAR2GO_LIB): %v", err)
	}
}

// Root is the module's root directory.
func Root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}
