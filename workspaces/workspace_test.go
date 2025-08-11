package workspaces_test

import (
	"context"
	"crypto/sha256"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("success_public_workspace", func(t *testing.T) {
		root := t.TempDir()
		moduleDir := "mymodule"
		moduleName := "test-module"
		require.NoError(t, os.Mkdir(filepath.Join(root, moduleDir), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(root, moduleDir, "main.go"), []byte("package main"), 0644))

		hasher := sha256.New()
		fileBytes, err := os.ReadFile(filepath.Join(root, moduleDir, "main.go"))
		require.NoError(t, err)
		_, err = hasher.Write(fileBytes)
		require.NoError(t, err)
		expectedCID := uuid.FromBytesOrNil(hasher.Sum(nil)).String()

		ws, err := workspaces.New(context.Background(), sha256.New(), root, moduleName)
		require.NoError(t, err)
		require.NotNil(t, ws)

		require.Equal(t, moduleName, ws.Module)
		require.Equal(t, root, ws.Root)
		require.Equal(t, filepath.Join(root, eg.ModuleDir), ws.ModuleDir)
		require.Equal(t, expectedCID, ws.CachedID)
		require.Equal(t, filepath.Join(root, eg.RuntimeDirectory), ws.RuntimeDir)
		require.Equal(t, filepath.Join(root, eg.WorkingDirectory), ws.WorkingDir)

		_, err = os.Stat(filepath.Join(root, eg.CacheDirectory))
		require.NoError(t, err, "CacheDir should be created")
		_, err = os.Stat(filepath.Join(root, ws.GenModDir))
		require.NoError(t, err, "GenModDir should be created")
		_, err = os.Stat(filepath.Join(root, ws.BuildDir, ws.Module, eg.ModuleDir))
		require.NoError(t, err, "BuildDir should be created")
		_, err = os.Stat(ws.RuntimeDir)
		require.NoError(t, err, "RuntimeDir should be created")
		_, err = os.Stat(ws.WorkspaceDir)
		require.NoError(t, err, "WorkspaceDir should be created")
	})

	t.Run("success_with_invalidate_cache_option", func(t *testing.T) {
		root := t.TempDir()
		moduleDir := "somemodule"
		moduleName := "test-invalidate"
		require.NoError(t, os.Mkdir(filepath.Join(root, moduleDir), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(root, moduleDir, "lib.go"), []byte("package lib"), 0644))

		hasher := sha256.New()
		fileBytes, err := os.ReadFile(filepath.Join(root, moduleDir, "lib.go"))
		require.NoError(t, err)
		_, err = hasher.Write(fileBytes)
		require.NoError(t, err)
		expectedCID := uuid.FromBytesOrNil(hasher.Sum(nil)).String()

		oldBuildDir := filepath.Join(root, eg.CacheDirectory, eg.DefaultModuleDirectory(), ".gen", expectedCID, "build")
		oldTransDir := filepath.Join(root, eg.CacheDirectory, eg.DefaultModuleDirectory(), ".gen", expectedCID, "trans")
		require.NoError(t, os.MkdirAll(oldBuildDir, 0755))
		require.NoError(t, os.MkdirAll(oldTransDir, 0755))
		dummyBuildFile := filepath.Join(oldBuildDir, "dummy.txt")
		dummyTransFile := filepath.Join(oldTransDir, "dummy.txt")
		require.NoError(t, os.WriteFile(dummyBuildFile, []byte("old"), 0644))
		require.NoError(t, os.WriteFile(dummyTransFile, []byte("old"), 0644))

		ws, err := workspaces.New(context.Background(), sha256.New(), root, moduleName, workspaces.OptionInvalidateModuleCache)
		require.NoError(t, err)
		require.NotNil(t, ws)
		require.Equal(t, expectedCID, ws.CachedID)

		_, err = os.Stat(dummyBuildFile)
		require.ErrorIs(t, err, os.ErrNotExist, "dummy file in build dir should be removed")
		_, err = os.Stat(dummyTransFile)
		require.ErrorIs(t, err, os.ErrNotExist, "dummy file in trans dir should be removed")

		_, err = os.Stat(filepath.Join(root, ws.BuildDir))
		require.NoError(t, err, "BuildDir should be recreated by ensuredirs")
		_, err = os.Stat(filepath.Join(root, ws.TransDir))
		require.NoError(t, err, "TransDir should be recreated by ensuredirs")
	})

	t.Run("success_with_option_enabled_false", func(t *testing.T) {
		root := t.TempDir()
		moduleName := "test-no-invalidate"
		require.NoError(t, os.WriteFile(filepath.Join(root, "lib.go"), []byte("package lib"), 0644))

		hasher := sha256.New()
		fileBytes, err := os.ReadFile(filepath.Join(root, "lib.go"))
		require.NoError(t, err)
		_, err = hasher.Write(fileBytes)
		require.NoError(t, err)
		expectedCID := uuid.FromBytesOrNil(hasher.Sum(nil)).String()

		oldBuildDir := filepath.Join(root, eg.CacheDirectory, eg.DefaultModuleDirectory(), ".gen", expectedCID, "build")
		dummyBuildFile := filepath.Join(oldBuildDir, "dummy.txt")
		require.NoError(t, os.MkdirAll(oldBuildDir, 0755))
		require.NoError(t, os.WriteFile(dummyBuildFile, []byte("old"), 0644))

		ws, err := workspaces.New(context.Background(), sha256.New(), root, moduleName, workspaces.OptionEnabled(workspaces.OptionInvalidateModuleCache, false))
		require.NoError(t, err)
		require.NotNil(t, ws)
		require.Equal(t, expectedCID, ws.CachedID)

		_, err = os.Stat(dummyBuildFile)
		require.NoError(t, err, "dummy file in build dir should NOT be removed")
	})

	t.Run("failure_on_invalid_permissions", func(t *testing.T) {
		roottmp := t.TempDir()
		require.NoError(t, os.Chmod(roottmp, 0444))
		root := filepath.Join(roottmp, "nonexistent")
		moduleName := "test-module"

		ws, err := workspaces.New(context.Background(), sha256.New(), root, moduleName)

		log.Println(spew.Sdump(ws))
		require.Error(t, err)
		require.ErrorIs(t, err, os.ErrPermission)
	})

	t.Run("success_with_nested_files", func(t *testing.T) {
		root := t.TempDir()
		moduleDir := "nestedmodule"
		moduleName := "test-nested"
		subDir := "internal"
		require.NoError(t, os.MkdirAll(filepath.Join(root, moduleDir, subDir), 0755))

		content1 := []byte("package internal")
		content2 := []byte("package main")
		path1 := filepath.Join(root, moduleDir, subDir, "logic.go")
		path2 := filepath.Join(root, moduleDir, "main.go")
		require.NoError(t, os.WriteFile(path1, content1, 0644))
		require.NoError(t, os.WriteFile(path2, content2, 0644))

		hasher := sha256.New()
		_, err := hasher.Write(content1) // fs.WalkDir is lexical: internal/logic.go
		require.NoError(t, err)
		_, err = hasher.Write(content2) // then main.go
		require.NoError(t, err)
		expectedCID := uuid.FromBytesOrNil(hasher.Sum(nil)).String()

		ws, err := workspaces.New(context.Background(), sha256.New(), root, moduleName)
		require.NoError(t, err)
		require.NotNil(t, ws)
		require.Equal(t, expectedCID, ws.CachedID)
	})

	t.Run("success_with_empty_module_dir", func(t *testing.T) {
		root := t.TempDir()
		moduleDir := "emptymodule"
		moduleName := "test-empty"
		require.NoError(t, os.Mkdir(filepath.Join(root, moduleDir), 0755))

		hasher := sha256.New()
		expectedCID := uuid.FromBytesOrNil(hasher.Sum(nil)).String()

		ws, err := workspaces.New(context.Background(), sha256.New(), root, moduleName)
		require.NoError(t, err)
		require.NotNil(t, ws)
		require.Equal(t, expectedCID, ws.CachedID)

		_, err = os.Stat(filepath.Join(root, ws.BuildDir, ws.Module, eg.ModuleDir))
		require.NoError(t, err, "BuildDir should be created even for empty module")
	})
}
