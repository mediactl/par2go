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
# Builds libpar2go.so inside debian:bookworm (glibc 2.36, clustarr's media
# image base) so it runs there and on any newer glibc. The image is pinned
# by digest and apt reads snapshot.debian.org at a fixed date, so the same
# commit builds with the same compiler every time.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="debian:bookworm@sha256:2c037a04925515fdd6ea85ea14a682d0e79931f5e9f5d07b6dbfc6ba12f9e858"
SNAPSHOT="20261001T000000Z"
# Runs as root for apt, then hands the outputs back to the caller.
docker run --rm -v "${root}:/src" -w /src "${IMAGE}" bash -c "
  set -e
  rm -f /etc/apt/sources.list.d/debian.sources
  printf 'deb [check-valid-until=no] http://snapshot.debian.org/archive/debian/${SNAPSHOT} bookworm main\\ndeb [check-valid-until=no] http://snapshot.debian.org/archive/debian-security/${SNAPSHOT} bookworm-security main\\n' > /etc/apt/sources.list
  apt-get update -qq
  apt-get install -y -qq --no-install-recommends cmake g++ git ca-certificates make >/dev/null
  git config --global --add safe.directory '*'  # the checkout is the caller's, not root's
  OUT_DIR=/src/shim/out CMAKE_BUILD_DIR=/src/shim/build/cmake-bookworm shim/build.sh
  chown -R $(id -u):$(id -g) /src/shim/build /src/shim/out
"
