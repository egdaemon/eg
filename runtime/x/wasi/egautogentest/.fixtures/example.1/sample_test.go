package sample

import "testing"

type fixture struct{}

func TestGreet(t *testing.T) {
	Greet(Account{Name: "world"})
}
