module github.com/akuityio/provider-crossplane-akuity

go 1.25

require (
	github.com/akuity/api-client-go v0.25.1
	github.com/akuity/grpc-gateway-client v0.0.0-20240912082144-55a48e8b4b89
	github.com/alecthomas/kingpin/v2 v2.4.0
	github.com/avast/retry-go/v4 v4.7.0
	github.com/crossplane/crossplane-runtime v1.20.0
	github.com/crossplane/crossplane-tools v0.0.0-20251017183449-dd4517244339
	github.com/google/go-cmp v0.7.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.11.1
	go.uber.org/mock v0.6.0
	google.golang.org/genproto/googleapis/api v0.0.0-20251022142026-3a174f9686a8
	k8s.io/apimachinery v0.34.1
	k8s.io/client-go v0.34.1
	k8s.io/kubectl v0.34.1
	// We have to pin to this version and not later until crossplane updates, otherwise there ends
	// up being two `Apply` methods in the crossplane-runtime module and it considers it an
	// ambiguous selector
	sigs.k8s.io/controller-runtime v0.21.0
	sigs.k8s.io/controller-tools v0.19.0
)

require (
	github.com/alevinval/sse v1.0.2 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/evanphx/json-patch v5.9.11+incompatible // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.1 // indirect
	github.com/go-openapi/swag/conv v0.25.1 // indirect
	github.com/go-openapi/swag/fileutils v0.25.1 // indirect
	github.com/go-openapi/swag/jsonname v0.25.1 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.1 // indirect
	github.com/go-openapi/swag/loading v0.25.1 // indirect
	github.com/go-openapi/swag/mangling v0.25.1 // indirect
	github.com/go-openapi/swag/netutils v0.25.1 // indirect
	github.com/go-openapi/swag/stringutils v0.25.1 // indirect
	github.com/go-openapi/swag/typeutils v0.25.1 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.1 // indirect
	github.com/go-resty/resty/v2 v2.11.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sync v0.17.0 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	k8s.io/code-generator v0.34.1 // indirect
	k8s.io/component-base v0.34.1 // indirect
	k8s.io/gengo/v2 v2.0.0-20250604051438-85fd79dbfd9f // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
)

require (
	dario.cat/mergo v1.0.1 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dave/jennifer v1.7.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.22.1 // indirect
	github.com/go-openapi/jsonreference v0.21.2 // indirect
	github.com/go-openapi/swag v0.25.1 // indirect
	github.com/gobuffalo/flect v1.0.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.2 // indirect
	github.com/prometheus/procfs v0.19.1 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/mod v0.28.0 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/oauth2 v0.32.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/term v0.36.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	golang.org/x/tools v0.37.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.5.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251014184007-4626949a642f // indirect
	google.golang.org/grpc v1.72.1
	google.golang.org/protobuf v1.36.10
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.34.1
	k8s.io/apiextensions-apiserver v0.34.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250910181357-589584f1c912 // indirect
	k8s.io/utils v0.0.0-20251002143259-bc988d571ff4
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/yaml v1.6.0
)

tool go.uber.org/mock/mockgen
