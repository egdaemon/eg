package actl

type AuthorizeAgent struct {
	Seed AuthorizeSecret `cmd:"" help:"register a signing secret"`
	ID   AuthorizeManual `cmd:"" help:"register using the id provided by the daemon without knowing the secret"`
}
