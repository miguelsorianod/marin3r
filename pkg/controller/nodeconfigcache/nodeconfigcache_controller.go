package nodeconfigcache

import (
	"context"
	"fmt"

	envoyapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyapi_auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	envoyapi_route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	envoyapi_discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/golang/protobuf/proto"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	cachesv1alpha1 "github.com/3scale/marin3r/pkg/apis/caches/v1alpha1"
	"github.com/3scale/marin3r/pkg/envoy"
	xds_cache_types "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	xds_cache "github.com/envoyproxy/go-control-plane/pkg/cache/v2"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	secretAnnotation  = "cert-manager.io/common-name"
	secretCertificate = "tls.crt"
	secretPrivateKey  = "tls.key"
)

var log = logf.Log.WithName("controller_nodeconfigcache")

// Add creates a new NodeConfigCache Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, cache *xds_cache.SnapshotCache) error {
	return add(mgr, newReconciler(mgr, cache))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, c *xds_cache.SnapshotCache) reconcile.Reconciler {
	return &ReconcileNodeConfigCache{
		client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		adsCache: c,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("nodeconfigcache-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource NodeConfigCache
	err = c.Watch(&source.Kind{Type: &cachesv1alpha1.NodeConfigCache{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// // Watch for changes to secondary resource Pods and requeue the owner NodeConfigCache
	// err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
	// 	IsController: true,
	// 	OwnerType:    &cachesv1alpha1.NodeConfigCache{},
	// })

	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileNodeConfigCache implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileNodeConfigCache{}

// ReconcileNodeConfigCache reconciles a NodeConfigCache object
type ReconcileNodeConfigCache struct {
	client   client.Client
	scheme   *runtime.Scheme
	adsCache *xds_cache.SnapshotCache
}

// Reconcile reads that state of the cluster for a NodeConfigCache object and makes changes based on the state read
// and what is in the NodeConfigCache.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNodeConfigCache) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling NodeConfigCache")

	ctx := context.TODO()

	// Fetch the NodeConfigCache instance
	instance := &cachesv1alpha1.NodeConfigCache{}
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	nodeID := instance.Spec.NodeID
	version := instance.Spec.Version
	snap := newNodeSnapshot(nodeID, version)

	switch serialization := instance.Spec.Serialization; serialization {
	case "b64json":
		ds := envoy.B64JSON{}
		err = r.loadResources(ctx, nodeID, instance, snap, ds)
	case "yaml":
		ds := envoy.YAML{}
		err = r.loadResources(ctx, nodeID, instance, snap, ds)
	default:
		// "json" is the default
		ds := envoy.JSON{}
		err = r.loadResources(ctx, nodeID, instance, snap, ds)
	}

	// Populate the snapshot with the resources in the spec
	if err != nil {
		// Return error to reenqueue
		// TODO: publish event
		// TODO: update condition
		reqLogger.Error(err, "Errors occured while loading resources from CR")
		return reconcile.Result{}, err
	}

	// Get the current published snapshot for this nodeID
	oldSnap, err := (*r.adsCache).GetSnapshot(nodeID)
	if err != nil {
		reqLogger.Info("No snapshot exists yet for the nodeID, creating a new one")
	} else if snapshotIsEqual(snap, &oldSnap) {
		reqLogger.Info("Generated snapshot is equal to published one, skipping reconcile")
		return reconcile.Result{}, nil
	}

	// TODO: check snapshot consistency using "snap.Consistent()". Consider also validating
	// consistency between clusters and listeners, as this is not done by snap.Consistent()
	// because listeners and clusters are not requested by name by the envoy gateways

	// Create a NodeConfigRevision for this config
	// createNewNodeConfigRevision()

	// Publish the in-memory cache to the envoy control-plane
	reqLogger.Info("Publishing new snapshot for nodeID", "Version", version)
	(*r.adsCache).SetSnapshot(nodeID, *snap)

	return reconcile.Result{}, nil
}

func resourceLoaderError(name, namespace, rtype, rvalue string, idx int) error {
	return errors.NewInvalid(
		schema.GroupKind{Group: "caches", Kind: "NodeCacheConfig"},
		fmt.Sprintf("%s/%s", namespace, name),
		field.ErrorList{
			field.Invalid(
				field.NewPath("Spec", rtype).Index(idx).Child("Value"),
				rvalue,
				fmt.Sprint("Invalid envoy resource value"),
			),
		},
	)
}

func (r *ReconcileNodeConfigCache) loadResources(ctx context.Context, nodeID string,
	o *cachesv1alpha1.NodeConfigCache, snap *xds_cache.Snapshot, ds envoy.ResourceUnmarshaller) error {

	for idx, endpoint := range o.Spec.Resources.Endpoints {
		res := &envoyapi.ClusterLoadAssignment{}
		if err := ds.Unmarshal(endpoint.Value, res); err != nil {
			return resourceLoaderError(o.ObjectMeta.Name, o.ObjectMeta.Namespace, "Endpoints", endpoint.Value, idx)
		}
		setResource(endpoint.Name, res, snap)
	}

	for idx, cluster := range o.Spec.Resources.Clusters {
		res := &envoyapi.Cluster{}
		if err := ds.Unmarshal(cluster.Value, res); err != nil {
			return resourceLoaderError(o.ObjectMeta.Name, o.ObjectMeta.Namespace, "Clusters", cluster.Value, idx)
		}
		setResource(cluster.Name, res, snap)
	}

	for idx, route := range o.Spec.Resources.Routes {
		res := &envoyapi_route.Route{}
		if err := ds.Unmarshal(route.Value, res); err != nil {
			return resourceLoaderError(o.ObjectMeta.Name, o.ObjectMeta.Namespace, "Routes", route.Value, idx)
		}
		setResource(route.Name, res, snap)
	}

	for idx, listener := range o.Spec.Resources.Listeners {
		res := &envoyapi.Listener{}
		if err := ds.Unmarshal(listener.Value, res); err != nil {
			return resourceLoaderError(o.ObjectMeta.Name, o.ObjectMeta.Namespace, "Listeners", listener.Value, idx)
		}
		setResource(listener.Name, res, snap)
	}

	for idx, runtime := range o.Spec.Resources.Runtimes {
		res := &envoyapi_discovery.Runtime{}
		if err := ds.Unmarshal(runtime.Value, res); err != nil {
			return resourceLoaderError(o.ObjectMeta.Name, o.ObjectMeta.Namespace, "Runtimes", runtime.Value, idx)
		}
		setResource(runtime.Name, res, snap)
	}

	for idx, secret := range o.Spec.Resources.Secrets {
		s := &corev1.Secret{}
		key := types.NamespacedName{
			Name:      secret.Ref.Name,
			Namespace: secret.Ref.Namespace,
		}
		if err := r.client.Get(ctx, key, s); err != nil {
			if errors.IsNotFound(err) {
				return err
			}
			return err
		}

		// Validate secret holds a certificate
		if s.Type == "kubernetes.io/tls" {
			res := envoy.NewSecret(secret.Name, string(s.Data[secretPrivateKey]), string(s.Data[secretCertificate]))
			setResource(secret.Name, res, snap)
		} else {
			return errors.NewInvalid(
				schema.GroupKind{Group: "caches", Kind: "NodeCacheConfig"},
				fmt.Sprintf("%s/%s", o.ObjectMeta.Namespace, o.ObjectMeta.Name),
				field.ErrorList{
					field.Invalid(
						field.NewPath("Spec", "Secrets").Index(idx).Child("Ref"),
						secret.Ref,
						fmt.Sprint("Only 'kubernetes.io/tls' type secrets allowed"),
					),
				},
			)
		}
	}

	return nil
}

func newNodeSnapshot(nodeID string, version string) *xds_cache.Snapshot {

	snap := xds_cache.Snapshot{Resources: [6]xds_cache.Resources{}}
	snap.Resources[xds_cache_types.Listener] = xds_cache.NewResources(version, []xds_cache_types.Resource{})
	snap.Resources[xds_cache_types.Endpoint] = xds_cache.NewResources(version, []xds_cache_types.Resource{})
	snap.Resources[xds_cache_types.Cluster] = xds_cache.NewResources(version, []xds_cache_types.Resource{})
	snap.Resources[xds_cache_types.Route] = xds_cache.NewResources(version, []xds_cache_types.Resource{})
	snap.Resources[xds_cache_types.Secret] = xds_cache.NewResources(version, []xds_cache_types.Resource{})
	snap.Resources[xds_cache_types.Runtime] = xds_cache.NewResources(version, []xds_cache_types.Resource{})

	return &snap
}

func setResource(name string, res xds_cache_types.Resource, snap *xds_cache.Snapshot) {

	switch o := res.(type) {

	case *envoyapi.ClusterLoadAssignment:
		snap.Resources[xds_cache_types.Endpoint].Items[name] = o

	case *envoyapi.Cluster:
		snap.Resources[xds_cache_types.Cluster].Items[name] = o

	case *envoyapi_route.Route:
		snap.Resources[xds_cache_types.Route].Items[name] = o

	case *envoyapi.Listener:
		snap.Resources[xds_cache_types.Listener].Items[name] = o

	case *envoyapi_auth.Secret:
		snap.Resources[xds_cache_types.Secret].Items[name] = o

	case *envoyapi_discovery.Runtime:
		snap.Resources[xds_cache_types.Runtime].Items[name] = o

	}
}

func snapshotIsEqual(newSnap, oldSnap *xds_cache.Snapshot) bool {

	// Check resources are equal for each resource type
	for rtype, newResources := range newSnap.Resources {
		oldResources := oldSnap.Resources[rtype]

		// If lenght is not equal, resources are not equal
		if len(oldResources.Items) != len(newResources.Items) {
			return false
		}

		for name, oldValue := range oldResources.Items {

			newValue, ok := newResources.Items[name]

			// If some key does not exist, resources are not equal
			if !ok {
				return false
			}

			// If value has changed, resources are not equal
			if !proto.Equal(oldValue, newValue) {
				return false
			}
		}
	}
	return true
}
