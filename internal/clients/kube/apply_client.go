package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubectl/pkg/scheme"
)

const (
	defaultDeleteWaitInterval = 500 * time.Millisecond
	defaultDeleteWaitTimeout  = 30 * time.Second
)

type ApplyClientOption func(*ApplyClient)

type ApplyClient struct {
	dynamicClient       dynamic.Interface
	clientset           kubernetes.Interface
	discoveryRestMapper meta.RESTMapper
	logger              logging.Logger
	deleteWaitInterval  time.Duration
	deleteWaitTimeout   time.Duration
}

type applyObject struct {
	name            string
	namespace       string
	unstructuredObj map[string]interface{}
}

func WithDeleteWaitTimeout(timeout time.Duration) ApplyClientOption {
	return func(a *ApplyClient) {
		if timeout > 0 {
			a.deleteWaitTimeout = timeout
		}
	}
}

func NewApplyClient(dynamicClient dynamic.Interface, clientset kubernetes.Interface, logger logging.Logger, opts ...ApplyClientOption) (*ApplyClient, error) {
	groupResources, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return &ApplyClient{}, fmt.Errorf("error setting up API discovery for dynamic client: %w", err)
	}

	ac := &ApplyClient{
		dynamicClient:       dynamicClient,
		clientset:           clientset,
		logger:              logger,
		discoveryRestMapper: restmapper.NewDiscoveryRESTMapper(groupResources),
		deleteWaitInterval:  defaultDeleteWaitInterval,
		deleteWaitTimeout:   defaultDeleteWaitTimeout,
	}
	for _, opt := range opts {
		opt(ac)
	}
	return ac, nil
}

func (a ApplyClient) ApplyManifests(ctx context.Context, manifests string, delete bool) error {
	separatedManifests := strings.Split(manifests, "---")
	objects, err := getObjectsFromManifests(separatedManifests)
	if err != nil {
		return err
	}

	for _, object := range objects {
		gvk := object.GetObjectKind().GroupVersionKind()
		gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
		mapping, err := a.discoveryRestMapper.RESTMapping(gk, gvk.Version)
		if err != nil {
			return fmt.Errorf("error setting up API mapping for dynamic client: %w", err)
		}

		applyObject, err := parseObject(object)
		if err != nil {
			return err
		}

		a.logger.Debug("Applying k8s object", "name", applyObject.name, "gvk", gvk)

		if delete {
			err = a.deleteObject(ctx, mapping, applyObject)
			if errors.IsNotFound(err) {
				err = nil
			}
		} else {
			err = a.applyObject(ctx, mapping, applyObject)
		}

		if err != nil {
			return fmt.Errorf("error applying resource: %w", err)
		}
	}

	return nil
}

func getObjectsFromManifests(manifests []string) ([]runtime.Object, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	objects := make([]runtime.Object, 0)

	for _, manifest := range manifests {
		if manifest == "" {
			continue
		}

		object, _, err := decode([]byte(manifest), nil, nil)
		if err != nil {
			return objects, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}

		objects = append(objects, object)
	}

	return objects, nil
}

func parseObject(object runtime.Object) (applyObject, error) {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		return applyObject{}, err
	}

	name, err := meta.NewAccessor().Name(object)
	if err != nil {
		return applyObject{}, err
	}

	namespace, err := meta.NewAccessor().Namespace(object)
	if err != nil {
		return applyObject{}, err
	}

	return applyObject{
		name:            name,
		namespace:       namespace,
		unstructuredObj: unstructuredObj,
	}, nil
}

func (a ApplyClient) deleteObject(ctx context.Context, mapping *meta.RESTMapping, applyObject applyObject) error {
	// Never delete a Namespace as part of manifest teardown. Agent
	// install manifests include a top-level Namespace so Apply can
	// create it when absent, but deleting it cascades unrelated tenant
	// resources and sibling agents in the same namespace.
	if mapping.Resource.Resource == "namespaces" {
		return nil
	}

	resource := a.resource(mapping, applyObject)
	// Foreground deletion keeps workload owners visible until their
	// dependents are gone; the wait below then covers pods spawned by an
	// agent Deployment, not just the Deployment object itself.
	propagation := v1.DeletePropagationForeground
	if err := resource.Delete(ctx, applyObject.name, v1.DeleteOptions{PropagationPolicy: &propagation}); err != nil {
		return err
	}

	return a.waitForDeleted(ctx, resource, mapping, applyObject)
}

func (a ApplyClient) resource(mapping *meta.RESTMapping, applyObject applyObject) dynamic.ResourceInterface {
	if isClusterScoped(mapping) {
		return a.dynamicClient.Resource(mapping.Resource)
	}
	return a.dynamicClient.Resource(mapping.Resource).Namespace(applyObject.namespace)
}

func (a ApplyClient) waitForDeleted(ctx context.Context, resource dynamic.ResourceInterface, mapping *meta.RESTMapping, applyObject applyObject) error {
	ref := objectRef(mapping, applyObject)
	err := wait.PollUntilContextTimeout(ctx, a.deleteWaitInterval, a.deleteWaitTimeout, true, func(ctx context.Context) (bool, error) {
		_, err := resource.Get(ctx, applyObject.name, v1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("wait for %s to be deleted within %s: %w", ref, a.deleteWaitTimeout, err)
	}
	return nil
}

func objectRef(mapping *meta.RESTMapping, applyObject applyObject) string {
	if isClusterScoped(mapping) {
		return fmt.Sprintf("%s/%s", mapping.Resource.Resource, applyObject.name)
	}
	return fmt.Sprintf("%s/%s/%s", mapping.Resource.Resource, applyObject.namespace, applyObject.name)
}

func (a ApplyClient) applyObject(ctx context.Context, mapping *meta.RESTMapping, applyObject applyObject) error {
	if isClusterScoped(mapping) {
		// Do not overwrite an existing namespace.
		if mapping.Resource.Resource == "namespaces" {
			_, err := a.dynamicClient.Resource(mapping.Resource).Get(ctx, applyObject.name, v1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					_, err = a.dynamicClient.Resource(mapping.Resource).Apply(ctx, applyObject.name, &unstructured.Unstructured{Object: applyObject.unstructuredObj}, v1.ApplyOptions{FieldManager: "application/apply-patch"})
					return err
				}

				return err
			}

			// Namespace already exists; patch metadata only.
			metadata := applyObject.unstructuredObj["metadata"]
			patchBytes, err := json.Marshal(metadata)
			if err != nil {
				return err
			}

			_, err = a.dynamicClient.Resource(mapping.Resource).Patch(ctx, applyObject.name, types.StrategicMergePatchType, patchBytes, v1.PatchOptions{FieldManager: "application/apply-patch"})
			return err
		}

		_, err := a.dynamicClient.Resource(mapping.Resource).Apply(ctx, applyObject.name, &unstructured.Unstructured{Object: applyObject.unstructuredObj}, v1.ApplyOptions{FieldManager: "application/apply-patch"})
		return err
	}

	_, err := a.dynamicClient.Resource(mapping.Resource).Namespace(applyObject.namespace).Apply(ctx, applyObject.name, &unstructured.Unstructured{Object: applyObject.unstructuredObj}, v1.ApplyOptions{FieldManager: "application/apply-patch"})
	return err
}

// isClusterScoped reports whether the RESTMapping targets a
// cluster-scoped resource. Trust discovery's REST scope instead of a
// hand-maintained allowlist; several install-time resources are
// cluster-scoped but not covered by the historical namespace/RBAC list.
func isClusterScoped(mapping *meta.RESTMapping) bool {
	return mapping.Scope != nil && mapping.Scope.Name() == meta.RESTScopeNameRoot
}
