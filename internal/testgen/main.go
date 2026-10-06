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

// Command testgen writes the fixtures' deterministic data and damage.
//
//	testgen data -seed N -size BYTES -o FILE
//	testgen damage -o FILE -bs BLOCKSIZE -first N -count M
package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: testgen data|damage ...")
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	out := fs.String("o", "", "file")
	seed := fs.Uint64("seed", 1, "PRNG seed")
	size := fs.Int("size", 0, "bytes")
	bs := fs.Int64("bs", 1024, "block size")
	first := fs.Int64("first", 0, "first block to damage")
	count := fs.Int64("count", 1, "blocks to damage")
	_ = fs.Parse(os.Args[2:])
	switch os.Args[1] {
	case "data":
		b := make([]byte, *size)
		r := rand.New(rand.NewPCG(*seed, *seed^0x9e3779b97f4a7c15))
		for i := range b {
			b[i] = byte(r.Uint32())
		}
		if err := os.WriteFile(*out, b, 0o644); err != nil {
			fail(err.Error())
		}
	case "damage":
		f, err := os.OpenFile(*out, os.O_RDWR, 0)
		if err != nil {
			fail(err.Error())
		}
		defer f.Close()
		junk := make([]byte, *bs)
		for i := range junk {
			junk[i] = 0xA5
		}
		for n := int64(0); n < *count; n++ {
			if _, err := f.WriteAt(junk, (*first+n)*(*bs)); err != nil {
				fail(err.Error())
			}
		}
	default:
		fail("unknown command " + os.Args[1])
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "testgen:", msg)
	os.Exit(2)
}
