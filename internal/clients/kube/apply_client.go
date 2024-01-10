package kube

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubectl/pkg/scheme"
)

type ApplyClient struct {
	dynamicClient       dynamic.Interface
	clientset           kubernetes.Interface
	discoveryRestMapper meta.RESTMapper
}

type applyObject struct {
	name            string
	namespace       string
	unstructuredObj map[string]interface{}
}

func NewApplyClient(dynamicClient dynamic.Interface, clientset kubernetes.Interface) (*ApplyClient, error) {
	groupResources, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return &ApplyClient{}, fmt.Errorf("error setting up API discovery for dynamic client: %w", err)
	}

	return &ApplyClient{
		dynamicClient:       dynamicClient,
		clientset:           clientset,
		discoveryRestMapper: restmapper.NewDiscoveryRESTMapper(groupResources),
	}, nil
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
	if isClusterScopedResource(mapping.Resource.Resource) {
		return a.dynamicClient.Resource(mapping.Resource).Delete(ctx, applyObject.name, v1.DeleteOptions{})
	}

	return a.dynamicClient.Resource(mapping.Resource).Namespace(applyObject.namespace).Delete(ctx, applyObject.name, v1.DeleteOptions{})
}

func (a ApplyClient) applyObject(ctx context.Context, mapping *meta.RESTMapping, applyObject applyObject) error {
	if isClusterScopedResource(mapping.Resource.Resource) {
		_, err := a.dynamicClient.Resource(mapping.Resource).Apply(ctx, applyObject.name, &unstructured.Unstructured{Object: applyObject.unstructuredObj}, v1.ApplyOptions{FieldManager: "application/apply-patch"})
		return err
	}

	_, err := a.dynamicClient.Resource(mapping.Resource).Namespace(applyObject.namespace).Apply(ctx, applyObject.name, &unstructured.Unstructured{Object: applyObject.unstructuredObj}, v1.ApplyOptions{FieldManager: "application/apply-patch"})

	return err
}

func isClusterScopedResource(resource string) bool {
	if resource == "namespaces" || resource == "clusterroles" || resource == "clusterrolebindings" {
		return true
	}

	return false
}
