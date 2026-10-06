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

// Package par2 verifies and repairs PAR2 sets through par2cmdline-turbo's
// library, loaded at run time from libpar2go.so with purego: no cgo.
//
// The library is found through $PAR2GO_LIB (a file path), then as
// libpar2go.so on the dynamic loader's search path.
//
// Each job runs on a thread of the library's own: no Go thread waits in C
// while it runs, and Go polls it for progress and completion. Jobs run one
// at a time per process. The library catches every C++ exception on the
// job's own thread, but par2's worker threads are its own: a crash or an
// uncaught exception there ends the process, so run untrusted sets in a
// process you can afford to lose.
package par2
