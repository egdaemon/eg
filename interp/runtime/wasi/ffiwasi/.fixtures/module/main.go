// module used by the module set tests in place of coreutils: a multicall binary that dispatches on its name.
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

// os.Args[0] selects the command:
//
//	cat [path...]       - copies the files, or stdin without any, to stdout.
//	tee path            - copies stdin to stdout and path.
//	write path content  - writes content to path.
//	env name            - prints the variable.
//	code n              - exits with n.
func main() {
	var (
		err error
	)

	switch os.Args[0] {
	case "cat":
		err = cat(os.Args[1:]...)
	case "tee":
		err = tee(os.Args[1])
	case "write":
		err = os.WriteFile(os.Args[1], []byte(os.Args[2]), 0600)
	case "env":
		fmt.Println(os.Getenv(os.Args[1]))
	case "code":
		code, _ := strconv.Atoi(os.Args[1])
		os.Exit(code)
	default:
		log.Fatalln("unknown command", os.Args[0])
	}

	if err != nil {
		log.Fatalln(err)
	}
}

func tee(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(io.MultiWriter(os.Stdout, f), os.Stdin)
	return err
}

func cat(paths ...string) error {
	if len(paths) == 0 {
		_, err := io.Copy(os.Stdout, os.Stdin)
		return err
	}

	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		os.Stdout.Write(b)
	}

	return nil
}
