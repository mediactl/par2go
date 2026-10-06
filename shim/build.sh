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
# Builds shim/out/libpar2go.so against par2cmdline-turbo at the pinned commit.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PAR2_TAG="v1.5.0-20261005"
PAR2_COMMIT="4aa390514d0236c00810f2a74f4435b6fff5df37"
work="${here}/build"
src="${work}/par2cmdline-turbo"
out="${OUT_DIR:-${here}/out}"

if [ ! -d "${src}/.git" ]; then
  git clone --quiet --depth 1 --branch "${PAR2_TAG}" \
    https://github.com/nzbgetcom/par2cmdline-turbo "${src}"
fi
got="$(git -C "${src}" rev-parse HEAD)"
if [ "${got}" != "${PAR2_COMMIT}" ]; then
  echo "par2cmdline-turbo ${PAR2_TAG} is ${got}, want ${PAR2_COMMIT}" >&2
  exit 1
fi

cmake -S "${here}" -B "${work}/cmake" -DCMAKE_BUILD_TYPE=Release -DPAR2_SRC="${src}"
cmake --build "${work}/cmake" --target par2go -j "$(nproc)"
mkdir -p "${out}"
install -m 0644 "${work}/cmake/libpar2go.so" "${out}/libpar2go.so"
echo "${out}/libpar2go.so"
