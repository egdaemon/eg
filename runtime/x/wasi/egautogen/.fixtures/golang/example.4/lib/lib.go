package lib

func Target() string { return "target" }

func Local() string {
	value := Target()
	return value
}
