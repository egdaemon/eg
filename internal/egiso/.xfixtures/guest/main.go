// guest used by the SysFS tests: runs one filesystem operation through WASI and prints its result.
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

// os.Args[1] selects the operation:
//
//	cat path                - prints the file.
//	write path content      - replaces the file's content, creating it.
//	append path content     - appends to the file, creating it.
//	pread path offset count - prints count bytes of the file from offset.
//	ls path                 - prints the directory's names, one per line.
//	regular path            - prints whether the file, following a symlink, is a regular file.
//	readlink path           - prints the symlink's target.
//	mkdir path              - creates the directory.
//	rm path                 - removes the file or empty directory.
//	mv from to              - renames from to to.
//	ln target path          - creates the symlink path to target.
func main() {
	var (
		err  error
		b    []byte
		args = os.Args[2:]
	)

	switch os.Args[1] {
	case "cat":
		if b, err = os.ReadFile(args[0]); err == nil {
			_, err = os.Stdout.Write(b)
		}
	case "write":
		err = os.WriteFile(args[0], []byte(args[1]), 0644)
	case "append":
		err = appendto(args[0], args[1])
	case "pread":
		err = pread(args[0], args[1], args[2])
	case "ls":
		err = ls(args[0])
	case "regular":
		var st os.FileInfo
		if st, err = os.Stat(args[0]); err == nil {
			fmt.Println(st.Mode().IsRegular())
		}
	case "readlink":
		var target string
		if target, err = os.Readlink(args[0]); err == nil {
			fmt.Println(target)
		}
	case "mkdir":
		err = os.Mkdir(args[0], 0755)
	case "rm":
		err = os.Remove(args[0])
	case "mv":
		err = os.Rename(args[0], args[1])
	case "ln":
		err = os.Symlink(args[0], args[1])
	default:
		err = fmt.Errorf("unknown operation %s", os.Args[1])
	}

	if err != nil {
		log.Fatalln(err)
	}
}

func appendto(name string, content string) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

func pread(name string, offset string, count string) error {
	off, err := strconv.ParseInt(offset, 10, 64)
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(count)
	if err != nil {
		return err
	}

	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	b := make([]byte, n)
	if _, err = f.ReadAt(b, off); err != nil && err != io.EOF {
		return err
	}
	_, err = os.Stdout.Write(b)
	return err
}

func ls(name string) error {
	entries, err := os.ReadDir(name)
	if err != nil {
		return err
	}
	for _, e := range entries {
		fmt.Println(e.Name())
	}
	return nil
}
