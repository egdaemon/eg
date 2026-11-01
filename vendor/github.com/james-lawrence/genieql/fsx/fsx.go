package fsx

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io/fs"
	"log"
	"os"
)

// IsRegularFile returns true IFF a non-directory file exists at the provided path.
func IsRegularFile(path string) bool {
	info, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	}

	if info.IsDir() {
		return false
	}

	return true
}

// MD5 computes digest of file contents.
// if something goes wrong logs and returns an empty string.
func MD5(path string) string {
	var (
		err  error
		read []byte
	)

	if read, err = os.ReadFile(path); err != nil {
		log.Println("digest failed", err)
		return ""
	}

	digest := md5.Sum(read)

	return hex.EncodeToString(digest[:])
}

func PrintFS(d fs.FS) {
	log.Println("--------- FS WALK INITIATED ---------")
	defer log.Println("--------- FS WALK COMPLETED ---------")

	err := fs.WalkDir(d, ".", func(path string, d fs.DirEntry, err error) error {
		// if err != nil {
		// 	return err
		// }

		log.Println(path)

		return nil
	})
	if err != nil {
		log.Println("fs walk failed", err)
	}
}

func PrintDir(d fs.FS) {
	log.Println("--------- FS WALK INITIATED ---------")
	defer log.Println("--------- FS WALK COMPLETED ---------")

	err := fs.WalkDir(d, ".", func(path string, d fs.DirEntry, err error) error {
		log.Println(path)

		if d.IsDir() && path != "." {
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		log.Println("fs walk failed", err)
	}
}

func PrintString(path string) {
	log.Println("--------- FS PRINT FILE INITIATED ---------")
	log.Println(path)
	defer log.Println("--------- FS PRINT FILE COMPLETED ---------")
	buf, err := os.ReadFile(path)
	if err != nil {
		log.Println("unable to read file", path, err)
		return
	}

	log.Printf("%s\n", bytes.NewBuffer(buf).String())
}
