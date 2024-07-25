package resource

import (
	"context"
	"fmt"
	"strings"
	"sync"

	libsveltosv1alpha1 "github.com/projectsveltos/libsveltos/api/v1alpha1"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"

	cleanyv1alpha1 "github.com/wys1203/Cleany/api/cleany/v1alpha1"
	"github.com/wys1203/Cleany/internal/executor/models"
)

// A interface for resource result helper
type IResourceHelper interface {
	// FetchMatchingResources fetches all resources matching the selector
	FetchMatchingResources(ctx context.Context) ([]models.ResourceResult, error)
}

type ResourceHelper struct {
	resourceSelectors []cleanyv1alpha1.ResourceSelector
	namespaces        []corev1.Namespace

	discoveryClient *discovery.DiscoveryClient
	dynamicClient   *dynamic.DynamicClient

	wg *sync.WaitGroup
}

func NewResourceHelper(
	resourceSelectors []cleanyv1alpha1.ResourceSelector,
	namespaces []corev1.Namespace,
	discoveryClient *discovery.DiscoveryClient,
	dynamicClient *dynamic.DynamicClient,
) IResourceHelper {
	return &ResourceHelper{
		resourceSelectors: resourceSelectors,
		namespaces:        namespaces,
		discoveryClient:   discoveryClient,
		dynamicClient:     dynamicClient,
		wg:                &sync.WaitGroup{},
	}
}

func (r *ResourceHelper) FetchMatchingResources(ctx context.Context) ([]models.ResourceResult, error) {

	// scan all resources group
	groupResources, err := restmapper.GetAPIGroupResources(r.discoveryClient)
	if err != nil {
		return nil, err
	}

	// init rest mapper
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	var resourceResults []models.ResourceResult

	for _, resourceSelector := range r.resourceSelectors {

		r.wg.Add(1)
		go func(resourceSelector cleanyv1alpha1.ResourceSelector) {
			defer r.wg.Done()

			// fetch resources
			resources, err := r.fetch(ctx, &resourceSelector, mapper)
			if err != nil {
				return
			}

			// match resources with selector evaluate
			for _, resource := range resources {
				match, message, err := resource.Match(resourceSelector.Evaluate)
				if err != nil {
					return
				}
				if match {
					resourceResults = append(resourceResults, models.ResourceResult{
						Resource: &resource.Unstructured,
						Message:  message,
					})
				}
			}

		}(resourceSelector)

	}

	r.wg.Wait()

	return resourceResults, nil
}

func (r *ResourceHelper) fetch(ctx context.Context, resourceSelector *cleanyv1alpha1.ResourceSelector, mapper meta.RESTMapper) ([]UnstructuredResource, error) {
	resourceId, err := constructGVR(resourceSelector, mapper)
	if err != nil {
		return nil, err
	}
	options := constructListOptions(labelFilter(resourceSelector), namespaceFilter(resourceSelector, r.namespaces))
	return collectWithOptions(ctx, &resourceId, &options, r.dynamicClient)
}

func constructGVR(resourceSelector *cleanyv1alpha1.ResourceSelector, mapper meta.RESTMapper) (schema.GroupVersionResource, error) {
	gvk := schema.GroupVersionKind{
		Group:   resourceSelector.Group,
		Version: resourceSelector.Version,
		Kind:    resourceSelector.Kind,
	}

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		if meta.IsNoMatchError(err) {
			return schema.GroupVersionResource{}, nil
		}
		return schema.GroupVersionResource{}, err
	}

	resourceId := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: mapping.Resource.Resource,
	}

	return resourceId, nil
}

func labelFilter(resourceSelector *cleanyv1alpha1.ResourceSelector) string {
	filters := make([]string, 0)
	for _, f := range resourceSelector.LabelFilters {
		if f.Operation == libsveltosv1alpha1.OperationEqual {
			filters = append(filters, fmt.Sprintf("%s=%s", f.Key, f.Value))
		} else {
			filters = append(filters, fmt.Sprintf("%s!=%s", f.Key, f.Value))
		}
	}
	return strings.Join(filters, ",")
}

func namespaceFilter(resourceSelector *cleanyv1alpha1.ResourceSelector, namespaces []corev1.Namespace) string {
	matchingNamespaces := make(map[string]struct{})
	if resourceSelector.NamespaceSelector != "" {
		parsedSelector, err := labels.Parse(resourceSelector.NamespaceSelector)
		if err != nil {
			return ""
		}
		for _, ns := range namespaces {
			if parsedSelector.Matches(labels.Set(ns.Labels)) {
				matchingNamespaces[ns.Name] = struct{}{}
			}
		}
	}
	if resourceSelector.Namespace != "" {
		matchingNamespaces[resourceSelector.Namespace] = struct{}{}
	}
	return strings.Join(maps.Keys(matchingNamespaces), ",")
}

func constructListOptions(labelFilter string, namespaceFilter string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: labelFilter,
		FieldSelector: fmt.Sprintf("metadata.namespace in (%s)", namespaceFilter),
	}
}

func collectWithOptions(ctx context.Context,
	resourceId *schema.GroupVersionResource,
	options *metav1.ListOptions,
	dynamicClient *dynamic.DynamicClient,
) ([]UnstructuredResource, error) {
	list, err := dynamicClient.Resource(*resourceId).List(ctx, *options)
	if err != nil {
		return nil, err
	}
	result := make([]UnstructuredResource, len(list.Items))
	for i, item := range list.Items {
		result[i] = UnstructuredResource{Unstructured: item}
	}
	return result, nil
}
