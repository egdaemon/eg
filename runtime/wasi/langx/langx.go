package langx

func Must[T any](v T, err error) T {
	if err == nil {
		return v
	}

	panic(err)
}
