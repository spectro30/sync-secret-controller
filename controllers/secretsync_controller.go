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
	"fmt"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	syncv1 "github.com/spectro30/sync-secret-controller/api/v1"
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
	if err := r.Get(ctx, req.NamespacedName, &secretSync); err != nil {
		log.Error(err, "unable to fetch SecretSync")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if secretSync.Spec.Paused != nil && *secretSync.Spec.Paused {
		log.V(1).Info("secretSync suspended, skipping")
		return ctrl.Result{}, nil
	}

	var sourceSecret core.Secret
	err := r.Get(ctx, types.NamespacedName{
		Namespace: secretSync.Spec.SourceRef.Namespace,
		Name:      secretSync.Spec.SourceRef.Name,
	}, &sourceSecret)

	if err != nil && !kerr.IsNotFound(err) {
		log.Error(err, "unable to fetch Secret")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	toBeDeleted := false
	if err != nil {
		toBeDeleted = true
	}

	createNameSpacedSecret := func(sourceSecret *core.Secret, namespace syncv1.NamespaceRef) (*core.Secret, error) {
		newSecret := &core.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sourceSecret.Name,
				Namespace: string(namespace),
			},
			Data: sourceSecret.Data,
		}
		return newSecret, nil
	}

	for _, namespace := range secretSync.Spec.TargetNamespaces {
		if toBeDeleted {
			var tempSecret core.Secret
			err := r.Get(ctx, types.NamespacedName{
				Namespace: string(namespace),
				Name:      secretSync.Spec.SourceRef.Name,
			}, &tempSecret)
			if err != nil {
				_ = r.Delete(ctx, &tempSecret)
				log.V(1).Info("delete secret for", "source secret", tempSecret.Name)
			}
		} else {
			newSecret, err := createNameSpacedSecret(&sourceSecret, namespace)
			if err != nil {
				log.Error(err, "unable to construct secret from template")
				// don't bother requeuing until we get a change to the spec
				return ctrl.Result{RequeueAfter: time.Minute}, nil
			}

			var tempSecret core.Secret
			err = r.Get(ctx, types.NamespacedName{
				Namespace: string(namespace),
				Name:      secretSync.Spec.SourceRef.Name,
			}, &tempSecret)

			if err == nil {
				log.V(1).Info("secret already present", "source secret", newSecret.Name)
			} else {
				if err := r.Create(ctx, newSecret); err != nil {
					log.Error(err, "unable to create secret for SecretSync")
				}
				log.V(1).Info("created secret for", "source secret", newSecret.Name)
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	secretHandler := handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
		fmt.Println("Secret Handler Summoned")
		secretSyncs := &syncv1.SecretSyncList{}
		if err := r.List(context.Background(), secretSyncs); err != nil {
			return nil
		}
		var req []reconcile.Request
		for _, c := range secretSyncs.Items {
			if c.Spec.SourceRef.Name == a.GetName() && c.Spec.SourceRef.Namespace == a.GetNamespace() {
				secret := &core.Secret{}

				if err := r.Get(context.Background(), types.NamespacedName{
					Namespace: a.GetNamespace(),
					Name:      a.GetName(),
				}, secret); err != nil {
					return nil
				}
				req = append(req, reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: c.Namespace,
					Name:      c.Name,
				}})
			}
		}
		return req
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&syncv1.SecretSync{}).
		Watches(&source.Kind{Type: &core.Secret{}}, secretHandler).
		Complete(r)
}
