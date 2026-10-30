package sample

type Public struct{}

type hiddenType struct{}

type Generic[T any] struct{ v T }

type hiddenGeneric[T any] struct{ v T }

func Exported() {}

func unexported() {}

func (t Public) Method() {}

func (t Public) hidden() {}

func (t hiddenType) Method() {}

func (t *Generic[T]) Method() {}

func (t hiddenGeneric[T]) Value() T { return t.v }
