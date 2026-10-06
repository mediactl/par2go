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
# image base) so it runs there and on any newer glibc.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# Runs as root for apt, then hands the outputs back to the caller.
docker run --rm -v "${root}:/src" -w /src debian:bookworm bash -c "
  set -e
  apt-get update -qq
  apt-get install -y -qq --no-install-recommends cmake g++ git ca-certificates make >/dev/null
  git config --global --add safe.directory '*'  # the checkout is the caller's, not root's
  OUT_DIR=/src/shim/out shim/build.sh
  chown -R $(id -u):$(id -g) /src/shim/build /src/shim/out
"
