package executor

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cleanyv1alpha1 "github.com/wys1203/Cleany/api/cleany/v1alpha1"
)

func Execute(ctx context.Context, cleaner *cleanyv1alpha1.Cleaner, client client.Client, scheme *runtime.Scheme, config *rest.Config) error {
	return nil
}
