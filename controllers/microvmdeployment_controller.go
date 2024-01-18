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
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/liquidmetal-dev/controller-pkg/types/microvm"
	"github.com/liquidmetal-dev/microvm-operator/api/v1alpha1"
	infrastructurev1alpha1 "github.com/liquidmetal-dev/microvm-operator/api/v1alpha1"
	infrav1 "github.com/liquidmetal-dev/microvm-operator/api/v1alpha1"
	"github.com/liquidmetal-dev/microvm-operator/internal/scope"
)

// MicrovmDeploymentReconciler reconciles a MicrovmDeployment object
type MicrovmDeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=infrastructure.liquid-metal.io,resources=microvmdeployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.liquid-metal.io,resources=microvmdeployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.liquid-metal.io,resources=microvmdeployments/finalizers,verbs=update
//+kubebuilder:rbac:groups=infrastructure.liquid-metal.io,resources=microvmreplicasets,verbs=get;list;watch;create;update;patch;delete

func (r *MicrovmDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	mvmD := &infrav1.MicrovmDeployment{}
	if err := r.Get(ctx, req.NamespacedName, mvmD); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		log.Error(err, "error getting microvmdeployment", "id", req.NamespacedName)

		return ctrl.Result{}, fmt.Errorf("unable to reconcile: %w", err)
	}

	mvmDeploymentScope, err := scope.NewMicrovmDeploymentScope(scope.MicrovmDeploymentScopeParams{
		MicrovmDeployment: mvmD,
		Client:            r.Client,
		Context:           ctx,
		Logger:            log,
	})
	if err != nil {
		log.Error(err, "failed to create mvm-deployment scope")

		return ctrl.Result{}, fmt.Errorf("failed to create mvm-deployment scope: %w", err)
	}

	defer func() {
		if err := mvmDeploymentScope.Patch(); err != nil {
			log.Error(err, "failed to patch microvmreplicaset")
		}
	}()

	if !mvmD.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("Deleting microvmdeployment")

		return r.reconcileDelete(ctx, mvmDeploymentScope)
	}

	return r.reconcileNormal(ctx, mvmDeploymentScope)
}

func (r *MicrovmDeploymentReconciler) reconcileDelete(
	ctx context.Context,
	mvmDeploymentScope *scope.MicrovmDeploymentScope,
) (reconcile.Result, error) {
	mvmDeploymentScope.Info("Reconciling MicrovmReplicaSet delete")

	// get all owned microvmreplicasets
	rsList, err := r.getOwnedReplicaSets(ctx, mvmDeploymentScope)
	if err != nil {
		mvmDeploymentScope.Error(err, "failed getting owned microvms")
		return ctrl.Result{}, fmt.Errorf("failed to list microvms: %w", err)
	}

	// if there are no owned sets left we are done, we can leave now
	if len(rsList) == 0 {
		controllerutil.RemoveFinalizer(mvmDeploymentScope.MicrovmDeployment, infrav1.MvmDeploymentFinalizer)
		mvmDeploymentScope.Info("microvmreplicaset deleted", "name", mvmDeploymentScope.Name())

		return ctrl.Result{}, nil
	}

	// there are still some resources to clear
	//
	// set the object to not ready before we remove anything
	mvmDeploymentScope.SetNotReady(infrav1.MicrovmDeploymentDeletingReason, "Info", "")
	// just to be complete, mark all replicas as not ready too
	mvmDeploymentScope.SetReadyReplicas(0)

	defer func() {
		if err := mvmDeploymentScope.Patch(); err != nil {
			mvmDeploymentScope.Error(err, "failed to patch microvmreplicaset")
		}
	}()

	var created int32 = 0

	for _, rs := range rsList {
		created += rs.Status.Replicas

		// if the object is already being deleted, skip this
		if !rs.DeletionTimestamp.IsZero() {
			continue
		}

		// otherwise send a delete call
		go func(rs infrav1.MicrovmReplicaSet) {
			if err := r.Delete(ctx, &rs); err != nil {
				mvmDeploymentScope.Error(err, "failed deleting microvmreplicaset", "set", rs.Name)
				mvmDeploymentScope.SetNotReady(infrav1.MicrovmDeploymentDeleteFailedReason, "Error", "")
			}
		}(rs)
	}

	// reset the number of still existing replicas, just so we know what is still there.
	// we'll come back around to ensure they are really gone.
	mvmDeploymentScope.SetCreatedReplicas(created)

	return ctrl.Result{RequeueAfter: requeuePeriod}, nil
}

func (r *MicrovmDeploymentReconciler) reconcileNormal(
	ctx context.Context,
	mvmDeploymentScope *scope.MicrovmDeploymentScope,
) (reconcile.Result, error) {
	mvmDeploymentScope.Info("Reconciling MicrovmDeployment update")

	// fetch all existing replicasets in this namespace
	rsList, err := r.getOwnedReplicaSets(ctx, mvmDeploymentScope)
	if err != nil {
		mvmDeploymentScope.Error(err, "failed getting owned microvms")

		return ctrl.Result{}, fmt.Errorf("failed to list microvms: %w", err)
	}

	defer func() {
		if err := mvmDeploymentScope.Patch(); err != nil {
			mvmDeploymentScope.Error(err, "unable to patch microvm")
		}
	}()

	// record the microvms per set which have been created and are ready
	// and create a map to record which host already has a replicaset

	// we always get a fresh count rather than rely on the status in case
	// something was removed
	var (
		ready   int32 = 0
		created int32 = 0

		activeHosts = v1alpha1.HostMap{}
		deadHosts   = v1alpha1.HostMap{}
	)

	for _, rs := range rsList {
		created += rs.Status.Replicas
		ready += rs.Status.ReadyReplicas

		activeHosts[rs.Spec.Host.Endpoint] = struct{}{}
		deadHosts[rs.Spec.Host.Endpoint] = struct{}{}
	}

	mvmDeploymentScope.SetCreatedReplicas(created)
	mvmDeploymentScope.SetReadyReplicas(ready)

	// get a count of the replicasets created
	createdSets := len(activeHosts)
	// check whether any hosts have been removed
	deadHosts = mvmDeploymentScope.ExpiredHosts(deadHosts)

	switch {
	// if all desired microvms are ready, mark the deployment ready.
	// we are done here
	case mvmDeploymentScope.ReadyReplicas() == mvmDeploymentScope.DesiredTotalReplicas():
		mvmDeploymentScope.Info("MicrovmDeployment created: ready")
		mvmDeploymentScope.SetReady()

		return reconcile.Result{}, nil
	// if we are here then a host has been removed.
	// we delete the set associated with that host.
	case len(deadHosts) > 0:
		mvmDeploymentScope.Info("MicrovmDeployment updating: delete microvmreplicaset")
		mvmDeploymentScope.SetNotReady(infrav1.MicrovmDeploymentUpdatingReason, "Info", "")

		for _, rs := range rsList {
			if _, ok := deadHosts[rs.Spec.Host.Endpoint]; !ok {
				continue
			}

			if !rs.DeletionTimestamp.IsZero() {
				return ctrl.Result{}, nil
			}

			if err := r.Delete(ctx, &rs); err != nil {
				mvmDeploymentScope.Error(err, "failed deleting microvmreplicaset")
				mvmDeploymentScope.SetNotReady(infrav1.MicrovmDeploymentUpdateFailedReason, "Error", "")

				return ctrl.Result{}, err
			}
		}
	// if we are in this branch then not all desired replicasets have been created.
	// create a new one and set the ownerref to this controller.
	case createdSets < mvmDeploymentScope.RequiredSets():
		mvmDeploymentScope.Info("MicrovmDeployment creating: create new microvmreplicaset")

		host, err := mvmDeploymentScope.DetermineHost(activeHosts)
		if err != nil {
			mvmDeploymentScope.Error(err, "failed creating owned microvmreplicaset")
			mvmDeploymentScope.SetNotReady(infrav1.MicrovmDeploymentProvisionFailedReason, "Error", "")

			return reconcile.Result{}, fmt.Errorf("failed to create new replicaset for deployment: %w", err)
		}

		if err := r.createReplicaSet(ctx, mvmDeploymentScope, host); err != nil {
			mvmDeploymentScope.Error(err, "failed creating owned microvmreplicaset")
			mvmDeploymentScope.SetNotReady(infrav1.MicrovmDeploymentProvisionFailedReason, "Error", "")

			return reconcile.Result{}, fmt.Errorf("failed to create new replicaset for deployment: %w", err)
		}

		mvmDeploymentScope.SetNotReady(infrav1.MicrovmDeploymentIncompleteReason, "Info", "")
	// if all desired objects have been created, but are not quite ready yet,
	// set the condition and requeue
	default:
		mvmDeploymentScope.Info("MicrovmReplicaSet creating: waiting for microvms to become ready")
		mvmDeploymentScope.SetNotReady(infrav1.MicrovmDeploymentIncompleteReason, "Info", "")
	}

	controllerutil.AddFinalizer(mvmDeploymentScope.MicrovmDeployment, infrav1.MvmDeploymentFinalizer)

	return ctrl.Result{RequeueAfter: requeuePeriod}, nil
}

func (r *MicrovmDeploymentReconciler) createReplicaSet(
	ctx context.Context,
	mvmDeploymentScope *scope.MicrovmDeploymentScope,
	host microvm.Host,
) error {
	newRs := &infrav1.MicrovmReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    mvmDeploymentScope.Namespace(),
			GenerateName: "microvmreplicaset-",
		},
		Spec: infrav1.MicrovmReplicaSetSpec{
			Host:     host,
			Replicas: pointer.Int32(mvmDeploymentScope.DesiredReplicas()),
			Template: infrav1.MicrovmTemplateSpec{
				Spec: mvmDeploymentScope.MicrovmSpec(),
			},
		},
	}

	if err := controllerutil.SetControllerReference(mvmDeploymentScope.MicrovmDeployment, newRs, r.Scheme); err != nil {
		return err
	}

	return r.Create(ctx, newRs)
}

func (r *MicrovmDeploymentReconciler) getOwnedReplicaSets(
	ctx context.Context,
	mvmDeploymentScope *scope.MicrovmDeploymentScope,
) ([]infrav1.MicrovmReplicaSet, error) {
	rsList := &infrav1.MicrovmReplicaSetList{}
	opts := []client.ListOption{
		client.InNamespace(mvmDeploymentScope.Namespace()),
	}
	if err := r.List(ctx, rsList, opts...); err != nil {
		return nil, err
	}

	owned := []v1alpha1.MicrovmReplicaSet{}

	for _, rs := range rsList.Items {
		if metav1.IsControlledBy(&rs, mvmDeploymentScope.MicrovmDeployment) {
			owned = append(owned, rs)
		}
	}

	return owned, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MicrovmDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.MicrovmDeployment{}).
		Owns(&infrav1.MicrovmReplicaSet{}).
		Complete(r)
}
