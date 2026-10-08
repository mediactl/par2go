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
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mediactl/par2go/internal/bindings"
)

// RequireLib skips t when libpar2shim.so cannot be loaded, unless
// PAR2GO_REQUIRE=1, which turns the skip into a failure (CI sets it).
func RequireLib(t testing.TB) {
	t.Helper()
	if err := bindings.Load(); err != nil {
		if os.Getenv("PAR2GO_REQUIRE") == "1" {
			t.Fatalf("PAR2GO_REQUIRE=1 but the library did not load: %v", err)
		}
		t.Skipf("libpar2shim.so unavailable (set PAR2GO_LIB): %v", err)
	}
}

// Root is the module's root directory.
func Root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// caseNames maps a committed testdata file name to the name CopyCase gives
// it. The unicode case's data file must be named "Amélie café – 2001.bin"
// to match its par2 set, but the Go module zip refuses a path element with an
// en dash (golang.org/x/mod/zip), which made v0.1.0 unfetchable as a module.
// So the file is committed under an ASCII name and renamed in the copy.
var caseNames = map[string]map[string]string{
	"unicode": {"unicode-name.bin": "Amélie café – 2001.bin"},
}

// CopyCase copies testdata/<name> into a fresh temp dir and returns it.
// Repairs write to the set, so a test never works on the committed copy.
func CopyCase(t testing.TB, name string) string {
	t.Helper()
	src := filepath.Join(Root(), "testdata", name)
	dst := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out := e.Name()
		if renamed, ok := caseNames[name][out]; ok {
			out = renamed
		}
		if err := os.WriteFile(filepath.Join(dst, out), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

// Originals maps each data file to the sha256 of its undamaged bytes.
func Originals(t testing.TB) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join(Root(), "testdata", "sha256.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m := map[string]string{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		if fields := strings.Fields(s.Text()); len(fields) == 2 {
			m[fields[1]] = fields[0]
		}
	}
	return m
}

// SHA256 is the hex sha256 of the file at path.
func SHA256(t testing.TB, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
