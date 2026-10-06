# par2go

PAR2 verify and repair for Go, without cgo: par2cmdline-turbo's library
behind a small C shim (`libpar2shim.so`), loaded at run time with
[purego](https://github.com/ebitengine/purego).

```go
res, err := par2.Repair(ctx, "/data/usenet/complete/job/set.par2", par2.Options{
	ExtraFiles: filesInDir, // matched by content, so renamed files are found
})
```

The library is found through `$PAR2GO_LIB`, then as `libpar2shim.so` on the
loader's path. Prebuilt `linux-amd64` and `linux-arm64` builds are attached
to each release; `shim/build.sh` builds one locally.

Licence: GPL-3.0-or-later (par2cmdline-turbo is GPL-2.0-or-later).

## Release assets

Each release attaches `libpar2shim-linux-amd64.so` and
`libpar2shim-linux-arm64.so`, built in `debian:bookworm` and tested before
upload, with `SHA256SUMS`. Each is self-contained: par2 and libstdc++ are
linked in, and only glibc is a dependency.

## Building the library

```sh
shim/build.sh              # host toolchain → shim/out/libpar2shim.so
shim/build-in-docker.sh    # debian:bookworm, as releases are built
```

`build.sh` fetches par2cmdline-turbo at a pinned commit and refuses any
other. Tests skip without the library unless `PAR2GO_REQUIRE=1`:

```sh
export PAR2GO_LIB=$PWD/shim/out/libpar2shim.so PAR2GO_REQUIRE=1
go test -race ./... && CGO_ENABLED=0 go test ./...
```

Both modes matter: `-race` needs cgo, while production runs
`CGO_ENABLED=0`, where purego uses its own fakecgo.

## Patches to par2cmdline-turbo

`shim/build.sh` applies `shim/patches/*.patch` on top of the pinned commit:

- `0001-keep-utf8-names.patch`: the nzbgetcom fork converts every stored
  file name from Latin-1 to UTF-8, so a name already written as UTF-8 (by
  par2cmdline, ParPar or MultiPar) was encoded twice and its file read as
  missing. Valid UTF-8 is now kept as is.
- `0002-scan-progress.patch`: the data scan never advanced the progress
  meter it was handed (`VerifyDataFile` called the `ScanDataFile` overload
  that makes a throwaway one), and that meter takes a callback upstream
  never set, so verification reported no progress. Both are fixed, through
  a new `SigScanProgress` hook.
