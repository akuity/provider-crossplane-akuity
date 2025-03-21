package cmp

import (
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func EquateEmpty() []cmp.Option {
	return []cmp.Option{
		cmpopts.EquateEmpty(),
		cmp.FilterValues(func(x, y any) bool {
			return isZeroValue(x) && isZeroValue(y)
		}, cmp.Ignore()),
	}
}

func isZeroValue(x any) bool { //nolint:gocyclo
	if x == nil {
		return true
	}
	v := reflect.ValueOf(x)
	switch v.Kind() { //nolint:exhaustive
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return true
		}
		if v.Elem().Kind() == reflect.Struct {
			return isZeroValue(v.Elem().Interface())
		}
		if v.Elem().Kind() == reflect.Bool {
			return !v.Elem().Bool()
		}
		return false
	case reflect.Slice, reflect.Map, reflect.Chan:
		return v.IsNil() || v.Len() == 0
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !isZeroValue(v.Field(i).Interface()) {
				return false
			}
		}
		return true
	default:
		return v.Interface() == reflect.Zero(v.Type()).Interface()
	}
}
