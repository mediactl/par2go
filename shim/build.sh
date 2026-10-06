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
# Builds shim/out/libpar2shim.so against par2cmdline-turbo at the pinned commit.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PAR2_TAG="v1.5.0-20261005"
PAR2_COMMIT="4aa390514d0236c00810f2a74f4435b6fff5df37"
work="${here}/build"
src="${work}/par2cmdline-turbo"
out="${OUT_DIR:-${here}/out}"
# A CMake cache records absolute paths, so the host and the bookworm
# container (which sees the tree at /src) each need their own.
cmakedir="${CMAKE_BUILD_DIR:-${work}/cmake}"

if [ ! -d "${src}/.git" ]; then
  git clone --quiet --depth 1 --branch "${PAR2_TAG}" \
    https://github.com/nzbgetcom/par2cmdline-turbo "${src}"
fi
got="$(git -C "${src}" rev-parse HEAD)"
if [ "${got}" != "${PAR2_COMMIT}" ]; then
  echo "par2cmdline-turbo ${PAR2_TAG} is ${got}, want ${PAR2_COMMIT}" >&2
  exit 1
fi

# Start every build from the pinned commit's own files, then apply
# par2go's patches (shim/patches), so local edits in the clone never
# reach a build.
git -C "${src}" checkout -q -- .
for p in "${here}"/patches/*.patch; do
  git -C "${src}" apply "${p}"
done

cmake -S "${here}" -B "${cmakedir}" -DCMAKE_BUILD_TYPE=Release -DPAR2_SRC="${src}"
cmake --build "${cmakedir}" --target par2shim par2 -j "$(nproc)"
mkdir -p "${out}"
install -m 0644 "${cmakedir}/libpar2shim.so" "${out}/libpar2shim.so"
install -m 0644 "${cmakedir}/libpar2.so" "${out}/libpar2.so"

# libpar2-dev: the patched headers, config.h and the flags libpar2.so was
# built with, for the C++ test that builds against it. Not published: a
# release carries only the .so files.
rm -rf "${out}/libpar2-dev"
mkdir -p "${out}/libpar2-dev/include"
cp -r "${src}/include/par2" "${out}/libpar2-dev/include/"
cp "${cmakedir}/par2-build/config.h" "${cmakedir}/par2-flags.txt" "${out}/libpar2-dev/"
echo "${out}/libpar2shim.so ${out}/libpar2.so"
