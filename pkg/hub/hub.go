package hub

import (
	"context"
	"embed"
	"github.com/pkg/errors"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/assets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	genericScheme = runtime.NewScheme()
	genericCodecs = serializer.NewCodecFactory(genericScheme)
	genericCodec  = genericCodecs.UniversalDeserializer()
)

func init() {
	utilruntime.Must(scheme.AddToScheme(genericScheme))
	utilruntime.Must(v1.AddToScheme(genericScheme))
}

//go:embed manifests
var fs embed.FS

var manifestFiles = []string{
	"manifests/service-account.yaml",
	"manifests/jaeger-deployment.yaml",
	"manifests/jaeger-service.yaml",
	"manifests/collector-deployment.yaml",
	"manifests/collector-service.yaml",
	"manifests/collector-config.yaml",
}

func Applymanifests(rclient client.Client) error {
	for _, file := range manifestFiles {
		template, err := fs.ReadFile(file)
		if err != nil {
			return err
		}
		raw := assets.MustCreateAssetFromTemplate(file, template, nil).Data
		obj, gvk, err := genericCodec.Decode(raw, nil, nil)
		if err != nil {
			klog.ErrorS(err, "Error decoding manifest file", "filename", file)
			return err
		}
		resource := obj.(client.Object)
		err = deploy(rclient, resource, *gvk)
		if err != nil {
			return err
		}
	}
	return nil
}

func deploy(rclient client.Client, resource client.Object, gvk schema.GroupVersionKind) error {

	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(gvk)
	if err := rclient.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: resource.GetNamespace(),
			Name:      resource.GetName(),
		}, current); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err,
				"failed to get obj kind: %s, namespace: %s, name %s",
				gvk.Kind,
				resource.GetNamespace(),
				resource.GetName(),
			)
		}
		// if not found, then create
		if err := rclient.Create(context.TODO(), resource); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return errors.Wrapf(err,
					"failed to create obj kind: %s, namespace: %s, name %s",
					gvk.Kind,
					resource.GetNamespace(),
					resource.GetName(),
				)
			}
		}
	}
	return nil
}
