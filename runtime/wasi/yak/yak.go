package yak

type Task interface {
	Do() error
}

type Op interface{}
type op func(Op) error

func Perform(tasks ...Task) {
	for _, t := range tasks {
		if err := t.Do(); err != nil {
			panic(err)
		}
	}
}

type fnTask func() error

func (t fnTask) Do() error {
	return t()
}

func Sequential(operations ...op) Task {
	return fnTask(func() error {
		for _, op := range operations {
			if err := op(nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func Parallel(operations ...op) Task {
	return fnTask(func() error {
		for _, op := range operations {
			if err := op(nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func When(b bool, o Task) Task {
	return fnTask(func() error {
		if !b {
			return nil
		}

		return o.Do()
	})
}
