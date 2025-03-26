package cmp

import (
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/resource"
)

func EquateEmpty() []cmp.Option {
	return []cmp.Option{
		cmpopts.EquateEmpty(),
		cmp.FilterValues(func(x, y any) bool {
			return isZeroValue(x) && isZeroValue(y)
		}, cmp.Ignore()),
		cmp.FilterValues(areEquivalentResourceQuantities, cmp.Ignore()),
	}
}

func areEquivalentResourceQuantities(x, y any) bool {
	xStr, xOk := x.(string)
	yStr, yOk := y.(string)
	if !xOk || !yOk {
		return false
	}

	xQuantity, xErr := resource.ParseQuantity(xStr)
	yQuantity, yErr := resource.ParseQuantity(yStr)
	if xErr != nil || yErr != nil {
		return false
	}

	return xQuantity.Equal(yQuantity)
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
			allZero := true
			for i := 0; i < v.Elem().NumField(); i++ {
				field := v.Elem().Field(i)
				if !isZeroValue(field.Interface()) {
					allZero = false
					break
				}
			}
			return allZero
		}
		if v.Elem().Kind() == reflect.Bool {
			return !v.Elem().Bool()
		}
		return isZeroValue(v.Elem().Interface())
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
