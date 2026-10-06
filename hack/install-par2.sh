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
# Installs the par2cmdline-turbo release clustarr's media image uses.
set -euo pipefail
ver="${PAR2_VERSION:-v1.5.0}"
case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64) arch=arm64 ;;
  *) echo "unsupported arch $(uname -m)" >&2; exit 1 ;;
esac
url="https://github.com/animetosho/par2cmdline-turbo/releases/download/${ver}/par2cmdline-turbo-${ver#v}-linux-${arch}.zip"
tmp="$(mktemp -d)"
curl -fsSL -o "${tmp}/par2.zip" "${url}"
unzip -p "${tmp}/par2.zip" par2 > "${tmp}/par2"
sudo install -m 0755 "${tmp}/par2" /usr/local/bin/par2
par2 --version | head -1
