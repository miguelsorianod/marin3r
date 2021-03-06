package controllers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *DiscoveryServiceReconciler) reconcileService(ctx context.Context) (reconcile.Result, error) {

	r.Log.V(1).Info("Reconciling Service")
	existent := &corev1.Service{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: OwnedObjectName(r.ds), Namespace: OwnedObjectNamespace(r.ds)}, existent)

	if err != nil {
		if errors.IsNotFound(err) {
			existent = r.genServiceObject()
			if err := controllerutil.SetControllerReference(r.ds, existent, r.Scheme); err != nil {
				return reconcile.Result{}, err
			}
			if err := r.Client.Create(ctx, existent); err != nil {
				return reconcile.Result{}, err
			}
			r.Log.Info("Created Service")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// We just reconcile the "Spec" field
	desired := r.genServiceObject()
	// ClusterIP is a field of the Spec populated by the Service controller
	desired.Spec.ClusterIP = existent.Spec.ClusterIP
	if !equality.Semantic.DeepEqual(existent.Spec, desired.Spec) {
		patch := client.MergeFrom(existent.DeepCopy())
		existent.Spec = desired.Spec
		if err := r.Client.Patch(ctx, existent, patch); err != nil {
			return reconcile.Result{}, err
		}
		r.Log.Info("Patched Service")
	}

	return reconcile.Result{}, nil
}

func (r *DiscoveryServiceReconciler) genServiceObject() *corev1.Service {

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OwnedObjectName(r.ds),
			Namespace: OwnedObjectNamespace(r.ds),
			Labels:    Labels(r.ds),
		},
		Spec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeClusterIP,
			Selector:        Labels(r.ds),
			SessionAffinity: corev1.ServiceAffinityNone,
			Ports: []corev1.ServicePort{
				{
					Name:       "discovery",
					Port:       18000,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString("discovery"),
				},
				{
					Name:       "webhook",
					Port:       443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString("webhook"),
				},
				{
					Name:       "metrics",
					Port:       8383,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString("metrics"),
				},
				{
					Name:       "cr-metrics",
					Port:       8686,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString("cr-metrics"),
				},
			},
		},
	}
}
