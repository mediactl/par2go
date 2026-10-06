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
#include "par2go.h"

extern "C" {

int32_t p2_abi_version(void) { return P2_ABI_VERSION; }

uint64_t p2_sizeof(int32_t which) {
  switch (which) {
  case P2_SIZEOF_PROGRESS: return sizeof(p2_progress);
  case P2_SIZEOF_COUNTS: return sizeof(p2_counts);
  case P2_SIZEOF_FILE: return sizeof(p2_file_result);
  }
  return 0;
}

} // extern "C"
