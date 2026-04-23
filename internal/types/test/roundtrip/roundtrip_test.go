// Package roundtrip_test locks the current converter behaviour as JSON
// golden snapshots. The snapshots in ./testdata/ must round-trip
// bit-identical across future codegen-emitter swaps. Refresh the
// snapshots with `-update`.
package roundtrip_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/akuityio/provider-crossplane-akuity/internal/controller/cluster"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/instance"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/observation"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/test/fixtures"
)

var update = flag.Bool("update", false, "rewrite testdata golden files")

const testdataDir = "testdata"

// TestCluster_CrossplaneToAkuityGolden snapshots the output of the
// Cluster controller's spec→API converter for fixtures.CrossplaneCluster.
// The codegen path must reproduce this snapshot byte-identical.
func TestCluster_CrossplaneToAkuityGolden(t *testing.T) {
	got, err := cluster.SpecToAPI(fixtures.CrossplaneCluster)
	require.NoError(t, err)
	assertGolden(t, "cluster_crossplane_to_akuity.json", got)
}

// TestCluster_AkuityToCrossplaneGolden snapshots the output of the
// Cluster controller's API→spec converter. The converter takes a spec
// it can late-initialise into, so feeding it fixtures.CrossplaneCluster
// is the intended call pattern in the production controller.
func TestCluster_AkuityToCrossplaneGolden(t *testing.T) {
	got, err := cluster.APIToSpec(
		fixtures.InstanceID,
		fixtures.CrossplaneCluster,
		fixtures.ArgocdCluster,
	)
	require.NoError(t, err)
	assertGolden(t, "cluster_akuity_to_crossplane.json", got)
}

// Instance round-trip runs at the InstanceSpec level. The current fixtures
// do not include a full *argocdv1.Instance + ExportInstanceResponse pair;
// they can be expanded later. For now spec-level conversion is the
// widest unit round-trip we can pin.
func TestInstanceSpec_CrossplaneToAkuityGolden(t *testing.T) {
	got, err := instance.SpecToInstanceSpec(fixtures.CrossplaneInstanceSpec)
	require.NoError(t, err)
	assertGolden(t, "instancespec_crossplane_to_akuity.json", got)
}

func TestInstanceSpec_AkuityToCrossplaneGolden(t *testing.T) {
	got, err := observation.InstanceArgoCDSpec(fixtures.ArgocdInstanceSpec)
	require.NoError(t, err)
	assertGolden(t, "instancespec_akuity_to_crossplane.json", got)
}

// assertGolden marshals v to stable JSON and compares it to the golden file
// at testdata/name. With -update, it writes the current value instead.
func assertGolden(t *testing.T, name string, v any) {
	t.Helper()
	got, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	got = append(got, '\n')

	path := filepath.Join(testdataDir, name)
	if *update {
		require.NoError(t, os.MkdirAll(testdataDir, 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))
		t.Logf("updated %s", path)
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden %s — run `go test ./internal/types/test/roundtrip -update`", path)
	if diff := cmp.Diff(string(want), string(got),
		cmpopts.EquateEmpty(),
		protocmp.Transform(),
	); diff != "" {
		t.Fatalf("golden mismatch for %s (-want +got):\n%s", name, diff)
	}
}
