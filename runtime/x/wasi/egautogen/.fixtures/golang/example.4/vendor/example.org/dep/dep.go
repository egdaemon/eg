package dep

import "example.com/callers/lib"

func Skipped() string { return lib.Target() }
