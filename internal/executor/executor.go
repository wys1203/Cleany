package executor

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cleanyv1alpha1 "github.com/wys1203/Cleany/api/cleany/v1alpha1"
	"github.com/wys1203/Cleany/internal/executor/resource"
)

type Executor struct {
	resourceHelper resource.IResourceHelper
}

func NewExecutor(
	ctx context.Context,
	cleanerName string,
	config *rest.Config,
	k8sClient client.Client,
	scheme *runtime.Scheme,
) (*Executor, error) {

	// Get the cleaner instance
	cleaner, err := getCleanerInstance(ctx, cleanerName, k8sClient)
	if err != nil {
		return nil, err
	}

	// Get the list of namespaces
	namespaces, err := getNamespaceList(ctx, k8sClient)
	if err != nil {
		return nil, err
	}

	// Create resource helper
	resourceHelper := resource.NewResourceHelper(
		cleaner.Spec.ResourcePolicySet.ResourceSelectors,
		namespaces,
		discovery.NewDiscoveryClientForConfigOrDie(config),
		dynamic.NewForConfigOrDie(config),
	)

	return &Executor{
		resourceHelper: resourceHelper,
	}, nil
}

func (e *Executor) Run(ctx context.Context) error {
	// Fetch all resources matching the selector
	resources, err := e.resourceHelper.FetchMatchingResources(ctx)
	if err != nil {
		return err
	}

	// Process each resource
	for _, resource := range resources {
		fmt.Println(resource)
	}

	return nil

}

func getCleanerInstance(ctx context.Context, cleanerName string, k8sClient client.Client) (*cleanyv1alpha1.Cleaner, error) {
	cleaner := new(cleanyv1alpha1.Cleaner)
	err := k8sClient.Get(ctx, types.NamespacedName{Name: cleanerName}, cleaner)
	if apierrors.IsNotFound(err) {
		err = nil
	}
	return cleaner, err
}

func getNamespaceList(ctx context.Context, k8sClient client.Client) ([]corev1.Namespace, error) {
	namespaceList := new(corev1.NamespaceList)
	if err := k8sClient.List(ctx, namespaceList); err != nil {
		return nil, err
	}
	result := make([]corev1.Namespace, 0)
	for _, namespace := range namespaceList.Items {
		if !namespace.DeletionTimestamp.IsZero() {
			continue
		}
		result = append(result, namespace)
	}
	return result, nil
}
