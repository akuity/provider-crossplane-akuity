package reason

import "errors"

type PermissionDenied struct {
	error
}

func AsPermissionDenied(err error) PermissionDenied {
	return PermissionDenied{err}
}

func IsPermissionDenied(err error) bool {
	return errors.Is(err, PermissionDenied{})
}

func (p PermissionDenied) Is(err error) bool {
	return err == PermissionDenied{}
}
