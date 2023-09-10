package osx

import (
	"log"
	"os"
)

func Getwd(fallback string) string {
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	} else {
		log.Println(err)
	}

	return fallback
}

func UserHomeDir(fallback string) string {
	if dir, err := os.UserHomeDir(); err == nil {
		return dir
	} else {
		log.Println(err)
	}

	return fallback
}
