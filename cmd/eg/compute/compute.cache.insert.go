package compute

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"charm.land/huh/v2"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/huhx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/internal/tarx"
)

type cacheInsert struct {
	cacheoptions `embed:""`
	Maximum      bytesx.Unit `name:"maximum" help:"uncompressed payload size above which the insert must be confirmed" default:"1GiB"`
	Paths        []string    `arg:"" name:"paths" type:"existingpath" help:"files and directories to insert into the cache"`
}

func (t cacheInsert) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, find libp2px.MDNSLookup) (err error) {
	var (
		tmpdir  string
		payload string
		size    bytesx.Unit
	)

	if tmpdir, err = os.MkdirTemp("", "eg.cache.insert.*"); err != nil {
		return errorsx.Wrap(err, "unable to create temporary directory")
	}
	defer func() {
		errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory"))
	}()

	if payload, size, err = cachePayload(tmpdir, t.Paths...); err != nil {
		return err
	}

	if err = cacheConfirmSize(size, t.Maximum, func(title string) (confirmed bool, err error) {
		prompt := huh.NewConfirm().Title(title).Value(&confirmed)
		notty := fmt.Errorf("payload %v exceeds --maximum %v, raise --maximum to insert when not interactive", size, t.Maximum)
		return confirmed, huhx.Prompter().NoTTY(notty).Run(prompt)
	}); err != nil {
		return err
	}

	log.Println("payload", payload, size)

	return t.cacheoptions.run(gctx, tlsc, ident, find, "cache/insert", fmt.Sprintf("cache insert - %v", size), nil, payload)
}

// cachePayload packs paths into the payload archive within dir, returning its path and the uncompressed size of its contents.
func cachePayload(dir string, paths ...string) (path string, size bytesx.Unit, err error) {
	var (
		payload *os.File
		n       int64
	)

	path = filepath.Join(dir, cachePayloadArchive)
	if payload, err = os.Create(path); err != nil {
		return "", 0, errorsx.Wrap(err, "unable to create the payload archive")
	}
	defer payload.Close()

	if err = tarx.Pack(payload, paths...); err != nil {
		return "", 0, errorsx.Wrap(err, "unable to pack the payload archive")
	}

	if err = iox.Rewind(payload); err != nil {
		return "", 0, errorsx.Wrap(err, "unable to rewind the payload archive")
	}

	if n, err = tarx.Size(payload); err != nil {
		return "", 0, errorsx.Wrap(err, "unable to compute the payload size")
	}

	return path, bytesx.Unit(n), nil
}

// cacheConfirmSize asks for confirmation when size exceeds maximum, returning an error unless it was confirmed.
func cacheConfirmSize(size, maximum bytesx.Unit, confirm func(title string) (bool, error)) error {
	if size <= maximum {
		return nil
	}

	confirmed, err := confirm(fmt.Sprintf("insert %v into the cache? (exceeds --maximum %v)", size, maximum))
	if err != nil {
		return err
	}

	if !confirmed {
		return errors.New("cache insert cancelled")
	}

	return nil
}
