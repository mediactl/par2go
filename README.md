# par2go

PAR2 verify and repair for Go, without cgo: par2cmdline-turbo's library
behind a small C shim (`libpar2go.so`), loaded at run time with
[purego](https://github.com/ebitengine/purego).

```go
res, err := par2.Repair(ctx, "/data/usenet/complete/job/set.par2", par2.Options{
	ExtraFiles: filesInDir, // matched by content, so renamed files are found
})
```

The library is found through `$PAR2GO_LIB`, then as `libpar2go.so` on the
loader's path. Prebuilt `linux-amd64` and `linux-arm64` builds are attached
to each release; `shim/build.sh` builds one locally.

Licence: GPL-3.0-or-later (par2cmdline-turbo is GPL-2.0-or-later).
