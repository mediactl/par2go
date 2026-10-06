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
# Installs the par2cmdline-turbo release clustarr's media image uses,
# checked against the digests recorded here (fetched 2026-10-06).
set -euo pipefail
ver="v1.5.0"
case "$(uname -m)" in
  x86_64) arch=amd64 sum=5a9f64386813456693c2ea1fb7649436fe7544bbdf97fd73b3483dfcc8aca464 ;;
  aarch64) arch=arm64 sum=3afb2f0b319fc4e6353c6d4261994757cbe3189f968a72420b4ba1d63f9c9a70 ;;
  *) echo "unsupported arch $(uname -m)" >&2; exit 1 ;;
esac
url="https://github.com/animetosho/par2cmdline-turbo/releases/download/${ver}/par2cmdline-turbo-${ver#v}-linux-${arch}.zip"
tmp="$(mktemp -d)"
curl -fsSL -o "${tmp}/par2.zip" "${url}"
echo "${sum}  ${tmp}/par2.zip" | sha256sum -c - >/dev/null
unzip -p "${tmp}/par2.zip" par2 > "${tmp}/par2"
sudo install -m 0755 "${tmp}/par2" /usr/local/bin/par2
par2 --version | head -1
