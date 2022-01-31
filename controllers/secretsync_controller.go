/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientutil "kmodules.xyz/client-go/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"

	syncv1 "github.com/spectro30/sync-secret-controller/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SecretSyncReconciler reconciles a SecretSync object
type SecretSyncReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=sync.spectro30,resources=secretsyncs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sync.spectro30,resources=secretsyncs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sync.spectro30,resources=secretsyncs/finalizers,verbs=update
//+kubebuilder:rbac:groups=v1,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=v1,resources=secrets/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SecretSync object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *SecretSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var secretSync syncv1.SecretSync
	err := r.Get(ctx, req.NamespacedName, &secretSync)
	if secretSync.Status.Synced != nil && *secretSync.Status.Synced == true {

	} else{
		log.Info("failed to get targeted secret or is not present")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if err != nil && errors.IsNotFound(err){


	} else if err != nil {

		return ctrl.Result{}, err
	}
	if secretSync.Spec.Paused != nil && *secretSync.Spec.Paused {
		log.V(1).Info("secretSync suspended, skipping")
		return ctrl.Result{}, nil
	}

	var sourceSecret core.Secret
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: secretSync.Spec.SourceRef.Namespace,
		Name:      secretSync.Spec.SourceRef.Name,
	}, &sourceSecret); err != nil {
		log.Info("unable to fetch Secret")
		return ctrl.Result{RequeueAfter: time.Minute}, client.IgnoreNotFound(err)
	}

	for _, namespace := range secretSync.Spec.TargetNamespaces {
		_, vtype, err := clientutil.CreateOrPatch(r.Client, &core.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sourceSecret.Name,
				Namespace: string(namespace),
			},
		}, func(obj client.Object, createOp bool) client.Object {
			secret := obj.(*core.Secret)
			secret.Labels = sourceSecret.Labels
			secret.Type = core.SecretTypeOpaque
			secret.Data = sourceSecret.Data
			return secret
		})

		if err != nil {
			log.Error(err, "config secret createorpatch failed")
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
		falseValue := false
		secretSync.Status.Synced = &falseValue

		log.Info("Secret type ", "is", vtype)

		log.V(1).Info("created secret for", "source secret", sourceSecret.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncv1.SecretSync{}).
		Watches(&source.Kind{Type: &core.Secret{}}, handler.EnqueueRequestsFromMapFunc(r.getHandlerFuncForSecret())).
		Complete(r)
}

func (r *SecretSyncReconciler) getHandlerFuncForSecret() handler.MapFunc {
	return func(object client.Object) []reconcile.Request {
		obj := object.(*core.Secret)
		var syncers syncv1.SecretSyncList
		var reqs []reconcile.Request

		// Listing from all namespaces
		err := r.Client.List(context.TODO(), &syncers, &client.ListOptions{})
		if err != nil {
			return reqs
		}
		for _, syncer := range syncers.Items {
			if syncer.Spec.SourceRef.Name == obj.Name {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      syncer.Name,
						Namespace: syncer.Namespace,
					},
				})
			}
		}
		return reqs
	}
}
