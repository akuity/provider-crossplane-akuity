package base

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
)

func TestPropagateObservedGeneration(t *testing.T) {
	mg := &v1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "inst",
			Namespace:  "ns",
			Generation: 42,
		},
	}

	PropagateObservedGeneration(mg)

	require.Equal(t, int64(42), mg.Status.ObservedGeneration)
}
