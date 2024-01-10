package reason

import "errors"

type NotFound struct {
	error
}

func AsNotFound(err error) NotFound {
	return NotFound{err}
}

func IsNotFound(err error) bool {
	return errors.Is(err, NotFound{})
}

func (n NotFound) Is(err error) bool {
	return err == NotFound{}
}
