package reason

import "errors"

type NotReconciled struct {
	error
}

func AsNotReconciled(err error) NotReconciled {
	return NotReconciled{err}
}

func IsNotReconciled(err error) bool {
	return errors.Is(err, NotReconciled{})
}

func (n NotReconciled) Is(err error) bool {
	return err == NotReconciled{}
}
