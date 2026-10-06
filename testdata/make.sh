#!/usr/bin/env bash
# Copyright 2026 The Clustarr Authors.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.
# Regenerates every fixture with the real par2 CLI. Run from the repo root.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
td="${root}/testdata"
gen() { (cd "${root}" && go run ./internal/testgen "$@"); }
command -v par2 >/dev/null || { echo "make.sh needs par2 on PATH" >&2; exit 1; }

rm -rf "${td}/base" "${td}"/{intact,repairable,unrepairable,renamed,volnames,unicode} "${td}/sha256.txt" "${td}/sha256-unicode.txt"
mkdir -p "${td}/base"
gen data -seed 1 -size 8192 -o "${td}/base/a.bin"
gen data -seed 2 -size 5000 -o "${td}/base/b.bin"
(cd "${td}/base" && par2 create -q -q -s1024 -c2 -n1 set.par2 a.bin b.bin)
(cd "${td}/base" && sha256sum a.bin b.bin) > "${td}/sha256.txt"

for c in intact repairable unrepairable renamed volnames; do
  cp -r "${td}/base" "${td}/${c}"
done
gen damage -o "${td}/repairable/a.bin" -bs 1024 -first 3 -count 1
gen damage -o "${td}/unrepairable/a.bin" -bs 1024 -first 0 -count 5
mv "${td}/renamed/a.bin" "${td}/renamed/z75QO.part070.rar"
gen damage -o "${td}/volnames/a.bin" -bs 1024 -first 3 -count 1
vol="$(cd "${td}/volnames" && ls set.vol*.par2)"
mv "${td}/volnames/${vol}" "${td}/volnames/set.vol-01.par2"
rm -rf "${td}/base"

# A set whose file name is UTF-8 with spaces, as usenet releases often are.
u="Amélie café – 2001.bin"
mkdir -p "${td}/unicode"
gen data -seed 3 -size 6144 -o "${td}/unicode/${u}"
(cd "${td}/unicode" && par2 create -q -q -s1024 -c2 -n1 set.par2 "${u}")
(cd "${td}/unicode" && sha256sum "${u}") > "${td}/sha256-unicode.txt"
gen damage -o "${td}/unicode/${u}" -bs 1024 -first 2 -count 1
ls -R "${td}"
