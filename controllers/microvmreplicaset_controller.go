/*
Copyright 2022 Liquid Metal Authors.

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/liquidmetal-dev/microvm-operator/api/v1alpha1"
	infrastructurev1alpha1 "github.com/liquidmetal-dev/microvm-operator/api/v1alpha1"
	infrav1 "github.com/liquidmetal-dev/microvm-operator/api/v1alpha1"
	"github.com/liquidmetal-dev/microvm-operator/internal/scope"
)

// MicrovmReplicaSetReconciler reconciles a MicrovmReplicaSet object
type MicrovmReplicaSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=infrastructure.liquid-metal.io,resources=microvmreplicasets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.liquid-metal.io,resources=microvmreplicasets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.liquid-metal.io,resources=microvmreplicasets/finalizers,verbs=update
//+kubebuilder:rbac:groups=infrastructure.liquid-metal.io,resources=microvms,verbs=get;list;watch;create;update;patch;delete

func (r *MicrovmReplicaSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	mvmRS := &infrav1.MicrovmReplicaSet{}
	if err := r.Get(ctx, req.NamespacedName, mvmRS); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		log.Error(err, "error getting microvmreplicaset", "id", req.NamespacedName)

		return ctrl.Result{}, fmt.Errorf("unable to reconcile: %w", err)
	}

	mvmReplicaSetScope, err := scope.NewMicrovmReplicaSetScope(scope.MicrovmReplicaSetScopeParams{
		MicrovmReplicaSet: mvmRS,
		Client:            r.Client,
		Context:           ctx,
		Logger:            log,
	})
	if err != nil {
		log.Error(err, "failed to create mvm-replicaset scope")

		return ctrl.Result{}, fmt.Errorf("failed to create mvm-replicaset scope: %w", err)
	}

	defer func() {
		if err := mvmReplicaSetScope.Patch(); err != nil {
			log.Error(err, "failed to patch microvmreplicaset")
		}
	}()

	if !mvmRS.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("Deleting microvmreplicaset")

		return r.reconcileDelete(ctx, mvmReplicaSetScope)
	}

	return r.reconcileNormal(ctx, mvmReplicaSetScope)
}

func (r *MicrovmReplicaSetReconciler) reconcileDelete(
	ctx context.Context,
	mvmReplicaSetScope *scope.MicrovmReplicaSetScope,
) (reconcile.Result, error) {
	mvmReplicaSetScope.Info("Reconciling MicrovmReplicaSet delete")

	// check the count of existing microvms and bail out early. we are done here.
	if mvmReplicaSetScope.CreatedReplicas() == 0 {
		controllerutil.RemoveFinalizer(mvmReplicaSetScope.MicrovmReplicaSet, infrav1.MvmRSFinalizer)
		mvmReplicaSetScope.Info("microvmreplicaset deleted", "name", mvmReplicaSetScope.Name())

		return ctrl.Result{}, nil
	}

	// there are still some resources to clear
	//
	// set the object to not ready before we remove anything
	mvmReplicaSetScope.SetNotReady(infrav1.MicrovmReplicaSetDeletingReason, "Info", "")
	// just to be complete, mark all replicas as not ready too
	mvmReplicaSetScope.SetReadyReplicas(0)

	defer func() {
		if err := mvmReplicaSetScope.Patch(); err != nil {
			mvmReplicaSetScope.Error(err, "failed to patch microvmreplicaset")
		}
	}()

	// get all owned microvms
	mvmList, err := r.getOwnedMicrovms(ctx, mvmReplicaSetScope)
	if err != nil {
		mvmReplicaSetScope.Error(err, "failed getting owned microvms")
		return ctrl.Result{}, fmt.Errorf("failed to list microvms: %w", err)
	}

	for _, mvm := range mvmList {
		// if the object is already being deleted, skip this
		if !mvm.DeletionTimestamp.IsZero() {
			continue
		}

		// otherwise send a delete call
		go func(m infrav1.Microvm) {
			if err := r.Delete(ctx, &m); err != nil {
				mvmReplicaSetScope.Error(err, "failed deleting microvm", "microvm", m.Name)
				mvmReplicaSetScope.SetNotReady(infrav1.MicrovmReplicaSetDeleteFailedReason, "Error", "")
			}
		}(mvm)
	}

	// reset the number of created replicas.
	// we'll come back around to ensure they are really gone.
	mvmReplicaSetScope.SetCreatedReplicas(int32(len(mvmList)))

	return ctrl.Result{RequeueAfter: requeuePeriod}, nil
}

func (r *MicrovmReplicaSetReconciler) reconcileNormal(
	ctx context.Context,
	mvmReplicaSetScope *scope.MicrovmReplicaSetScope,
) (reconcile.Result, error) {
	mvmReplicaSetScope.Info("Reconciling MicrovmReplicaSet update")

	// fetch all existing microvms in this rs namespace
	mvmList, err := r.getOwnedMicrovms(ctx, mvmReplicaSetScope)
	if err != nil {
		mvmReplicaSetScope.Error(err, "failed getting owned microvms")

		return ctrl.Result{}, fmt.Errorf("failed to list microvms: %w", err)
	}

	defer func() {
		if err := mvmReplicaSetScope.Patch(); err != nil {
			mvmReplicaSetScope.Error(err, "unable to patch microvm")
		}
	}()

	// record which owned replicas have been created
	// we always get a fresh count rather than rely on the RS status in case
	// something was removed
	mvmReplicaSetScope.SetCreatedReplicas(int32(len(mvmList)))

	var ready int32 = 0
	for _, mvm := range mvmList {
		if mvm.Status.Ready {
			ready++
		}
	}

	// record which owned replicas are ready
	mvmReplicaSetScope.SetReadyReplicas(ready)

	switch {
	// if all desired microvms are ready, mark the replicaset ready.
	// we are done here
	case mvmReplicaSetScope.ReadyReplicas() == mvmReplicaSetScope.DesiredReplicas():
		mvmReplicaSetScope.Info("MicrovmReplicaSet created: ready")
		mvmReplicaSetScope.SetReady()

		return reconcile.Result{}, nil
	// if we are in this branch then not all desired microvms have been created.
	// create a new one and set the ownerref to this controller.
	case mvmReplicaSetScope.CreatedReplicas() < mvmReplicaSetScope.DesiredReplicas():
		mvmReplicaSetScope.Info("MicrovmReplicaSet creating: create new microvm")

		if err := r.createMicrovm(ctx, mvmReplicaSetScope); err != nil {
			mvmReplicaSetScope.Error(err, "failed creating owned microvm")
			mvmReplicaSetScope.SetNotReady(infrav1.MicrovmReplicaSetProvisionFailedReason, "Error", "")

			return reconcile.Result{}, fmt.Errorf("failed to create new microvm for replicaset: %w", err)
		}

		mvmReplicaSetScope.SetNotReady(infrav1.MicrovmReplicaSetIncompleteReason, "Info", "")
	// if we are here then a scale down has been requested.
	// we delete the first found until the numbers balance out.
	// TODO the way this works is very naive and often ends up deleting everything
	// if the timing is wrong/right, find a better way https://github.com/liquidmetal-dev/microvm-operator/issues/17
	case mvmReplicaSetScope.CreatedReplicas() > mvmReplicaSetScope.DesiredReplicas():
		mvmReplicaSetScope.Info("MicrovmReplicaSet updating: delete microvm")
		mvmReplicaSetScope.SetNotReady(infrav1.MicrovmReplicaSetUpdatingReason, "Info", "")

		mvm := mvmList[0]
		if !mvm.DeletionTimestamp.IsZero() {
			return ctrl.Result{}, nil
		}

		if err := r.Delete(ctx, &mvm); err != nil {
			mvmReplicaSetScope.Error(err, "failed deleting microvm")
			mvmReplicaSetScope.SetNotReady(infrav1.MicrovmReplicaSetDeleteFailedReason, "Error", "")

			return ctrl.Result{}, err
		}
	// if all desired microvms have been created, but are not quite ready yet,
	// set the condition and requeue
	default:
		mvmReplicaSetScope.Info("MicrovmReplicaSet creating: waiting for microvms to become ready")
		mvmReplicaSetScope.SetNotReady(infrav1.MicrovmReplicaSetIncompleteReason, "Info", "")
	}

	controllerutil.AddFinalizer(mvmReplicaSetScope.MicrovmReplicaSet, infrav1.MvmRSFinalizer)

	return ctrl.Result{RequeueAfter: requeuePeriod}, nil
}

func (r *MicrovmReplicaSetReconciler) createMicrovm(
	ctx context.Context,
	mvmReplicaSetScope *scope.MicrovmReplicaSetScope,
) error {
	newMvm := &infrav1.Microvm{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    mvmReplicaSetScope.Namespace(),
			GenerateName: "microvm-",
		},
		Spec: mvmReplicaSetScope.MicrovmSpec(),
	}
	newMvm.Spec.Host = mvmReplicaSetScope.MicrovmHost()

	if err := controllerutil.SetControllerReference(mvmReplicaSetScope.MicrovmReplicaSet, newMvm, r.Scheme); err != nil {
		return err
	}

	return r.Create(ctx, newMvm)
}

func (r *MicrovmReplicaSetReconciler) getOwnedMicrovms(
	ctx context.Context,
	mvmReplicaSetScope *scope.MicrovmReplicaSetScope,
) ([]infrav1.Microvm, error) {
	mvmList := &infrav1.MicrovmList{}
	opts := []client.ListOption{
		client.InNamespace(mvmReplicaSetScope.Namespace()),
	}
	if err := r.List(ctx, mvmList, opts...); err != nil {
		return nil, err
	}

	owned := []v1alpha1.Microvm{}

	for _, mvm := range mvmList.Items {
		if metav1.IsControlledBy(&mvm, mvmReplicaSetScope.MicrovmReplicaSet) {
			owned = append(owned, mvm)
		}
	}

	return owned, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MicrovmReplicaSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.MicrovmReplicaSet{}).
		Owns(&infrastructurev1alpha1.Microvm{}).
		Complete(r)
}
