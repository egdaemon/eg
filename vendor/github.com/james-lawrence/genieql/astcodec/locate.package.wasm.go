//go:build wasm

package astcodec

import (
	"encoding/json"
	"go/build"
	"unsafe"

	"github.com/james-lawrence/genieql/internal/bytesx"
	"github.com/james-lawrence/genieql/internal/errorsx"
	"github.com/james-lawrence/genieql/internal/wasix/ffiguest"
)

// LocatePackage finds a package by its name.
func LocatePackage(importPath, srcDir string, context build.Context, matches func(*build.Package) bool) (pkg *build.Package, err error) {
	var (
		rs = make([]byte, 0, 2*bytesx.MiB)
	)

	ipathptr, ipathlen := ffiguest.String(importPath)
	srcdptr, srcdlen := ffiguest.String(srcDir)
	tagsptr, tagslen, tagssize := ffiguest.StringArray(context.BuildTags...)
	_, rptr, rlen := ffiguest.ByteBuffer(rs)
	err = ffiguest.Error(
		_locatepackage(
			ipathptr, ipathlen,
			srcdptr, srcdlen,
			tagsptr, tagslen, tagssize,
			unsafe.Pointer(&rlen), rptr,
		),
		errorsx.String("locate package failed"),
	)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(ffiguest.ByteBufferRead(rptr, rlen), &pkg); err != nil {
		return nil, err
	}

	if pkg != nil && (matches == nil || matches(pkg)) {
		return pkg, nil
	}

	return nil, ErrPackageNotFound
}

//go:wasmimport env genieql/astcodec.LocatePackage
func _locatepackage(
	ipathptr unsafe.Pointer, ipathptrlen uint32,
	srcdirptr unsafe.Pointer, srcdirlen uint32,
	tagsptr unsafe.Pointer, tagslen uint32, tagssize uint32,
	rlen unsafe.Pointer,
	rptr unsafe.Pointer,
) (errcode uint32)
