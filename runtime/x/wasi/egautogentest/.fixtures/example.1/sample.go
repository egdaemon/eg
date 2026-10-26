package sample

type Account struct {
	Name string
}

func Greet(a Account) string {
	return "hello " + a.Name
}

func (t *Account) Rename(n string) {
	t.Name = n
}
