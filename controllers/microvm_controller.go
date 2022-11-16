/*
Copyright 2022 Weaveworks.

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
	"strings"
	"time"

	flclient "github.com/weaveworks-liquidmetal/controller-pkg/client"
	flintlocktypes "github.com/weaveworks-liquidmetal/flintlock/api/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/weaveworks-liquidmetal/microvm-operator/api/v1alpha1"
	"github.com/weaveworks-liquidmetal/microvm-operator/internal/scope"
	"github.com/weaveworks-liquidmetal/microvm-operator/internal/services/flintlock"
)

const (
	requeuePeriod = 30 * time.Second
)

// MicrovmReconciler reconciles a Microvm object
type MicrovmReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	MvmClientFunc flclient.FactoryFunc
}

//+kubebuilder:rbac:groups=infrastructure.flintlock.x-k8s.io,resources=microvms,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.flintlock.x-k8s.io,resources=microvms/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.flintlock.x-k8s.io,resources=microvms/finalizers,verbs=update

func (r *MicrovmReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	mvm := &infrav1.Microvm{}
	if err := r.Get(ctx, req.NamespacedName, mvm); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		log.Error(err, "error getting microvmmachine", "id", req.NamespacedName)

		return ctrl.Result{}, fmt.Errorf("unable to reconcile: %w", err)
	}

	mvmScope, err := scope.NewMicrovmScope(scope.MicrovmScopeParams{
		MicroVM: mvm,
		Client:  r.Client,
		Context: ctx,
		Logger:  log,
	})
	if err != nil {
		log.Error(err, "failed to create machine scope")

		return ctrl.Result{}, fmt.Errorf("failed to create machine scope: %w", err)
	}

	defer func() {
		if patchErr := mvmScope.Patch(); patchErr != nil {
			log.Error(patchErr, "failed to patch microvm")
		}
	}()

	if !mvm.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("Deleting microvm")

		return r.reconcileDelete(ctx, mvmScope)
	}

	return r.reconcileNormal(ctx, mvmScope)
}

func (r *MicrovmReconciler) reconcileDelete(
	ctx context.Context,
	machineScope *scope.MicrovmScope,
) (reconcile.Result, error) {
	machineScope.Info("Reconciling MicrovmMachine delete")

	mvmSvc, err := r.getMicrovmService(machineScope)
	if err != nil {
		machineScope.Error(err, "failed to get microvm service")

		return ctrl.Result{}, nil
	}
	defer mvmSvc.Dispose()

	microvm, err := mvmSvc.Get(ctx)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		machineScope.Error(err, "failed getting microvm")

		return ctrl.Result{}, fmt.Errorf("failed getting microvm: %w", err)
	}

	if microvm != nil {
		machineScope.Info("deleting microvm")

		// Mark the machine as no longer ready before we delete.
		machineScope.SetNotReady(infrav1.MicrovmDeletingReason, "Info", "")

		if err := machineScope.Patch(); err != nil {
			machineScope.Error(err, "failed to patch object")

			return ctrl.Result{}, err
		}

		if microvm.Status.State != flintlocktypes.MicroVMStatus_DELETING {
			if _, err := mvmSvc.Delete(ctx); err != nil {
				machineScope.SetNotReady(infrav1.MicrovmDeleteFailedReason, "Error", "")

				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{RequeueAfter: requeuePeriod}, nil
	}

	// By this point Flintlock has no record of the MvM, so we are good to clear
	// the finalizer
	controllerutil.RemoveFinalizer(machineScope.MicroVM, infrav1.MvmFinalizer)
	machineScope.Info("microvm deleted")

	return ctrl.Result{}, nil
}

func (r *MicrovmReconciler) reconcileNormal(
	ctx context.Context,
	mvmScope *scope.MicrovmScope,
) (reconcile.Result, error) {
	mvmSvc, err := r.getMicrovmService(mvmScope)
	if err != nil {
		mvmScope.Error(err, "failed to get microvm service")

		return ctrl.Result{}, err
	}
	defer mvmSvc.Dispose()

	var microvm *flintlocktypes.MicroVM

	providerID := mvmScope.GetProviderID()
	if providerID != "" {
		var err error

		microvm, err = mvmSvc.Get(ctx)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			mvmScope.Error(err, "failed checking if microvm exists")

			return ctrl.Result{}, err
		}
	}

	controllerutil.AddFinalizer(mvmScope.MicroVM, infrav1.MvmFinalizer)

	if err := mvmScope.Patch(); err != nil {
		mvmScope.Error(err, "unable to patch microvm machine")

		return ctrl.Result{}, err
	}

	if microvm == nil {
		mvmScope.Info("creating microvm")

		microvm, err = mvmSvc.Create(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	mvmScope.SetProviderID(*microvm.Spec.Uid)

	if err := mvmScope.Patch(); err != nil {
		mvmScope.Error(err, "unable to patch microvm machine")

		return ctrl.Result{}, err
	}

	return r.parseMicroVMState(mvmScope, microvm.Status.State)
}

func (r *MicrovmReconciler) getMicrovmService(
	mvmScope *scope.MicrovmScope,
) (*flintlock.Service, error) {
	if r.MvmClientFunc == nil {
		return nil, errClientFactoryFuncRequired
	}

	// token, err := mvmScope.GetBasicAuthToken(addr)
	// if err != nil {
	// 	return nil, fmt.Errorf("getting basic auth token: %w", err)
	// }

	// tls, err := mvmScope.GetTLSConfig()
	// if err != nil {
	// 	return nil, fmt.Errorf("getting tls config: %w", err)
	// }

	// clientOpts := []flclient.Options{
	// 	flclient.WithProxy(machineScope.MvmCluster.Spec.MicrovmProxy),
	// 	flclient.WithBasicAuth(token),
	// 	flclient.WithTLS(tls),
	// }

	client, err := r.MvmClientFunc(mvmScope.MicroVM.Spec.Host.Endpoint, []flclient.Options{}...)
	if err != nil {
		return nil, fmt.Errorf("creating microvm client: %w", err)
	}

	return flintlock.New(mvmScope, client, mvmScope.MicroVM.Spec.Host.Endpoint), nil
}

func (r *MicrovmReconciler) parseMicroVMState(
	mvmScope *scope.MicrovmScope,
	state flintlocktypes.MicroVMStatus_MicroVMState,
) (ctrl.Result, error) {
	switch state {
	// ALL DONE \o/
	case flintlocktypes.MicroVMStatus_CREATED:
		mvmScope.MicroVM.Status.VMState = &infrav1.VMStateRunning
		mvmScope.V(2).Info("microvm is in created state")
		mvmScope.Info("microvm created", "name", mvmScope.Name(), "UID", mvmScope.GetInstanceID())
		mvmScope.SetReady()

		return reconcile.Result{}, nil
	// MVM IS PENDING
	case flintlocktypes.MicroVMStatus_PENDING:
		mvmScope.MicroVM.Status.VMState = &infrav1.VMStatePending
		mvmScope.SetNotReady(infrav1.MicrovmPendingReason, "Info", "")

		return ctrl.Result{RequeueAfter: requeuePeriod}, nil
	// MVM IS FAILING
	case flintlocktypes.MicroVMStatus_FAILED:
		// TODO: we need a failure reason from flintlock: Flintlock #299
		mvmScope.MicroVM.Status.VMState = &infrav1.VMStateFailed
		mvmScope.SetNotReady(infrav1.MicrovmProvisionFailedReason,
			"Error",
			errMicrovmFailed.Error(),
		)

		return ctrl.Result{}, errMicrovmFailed
	// MVM RECEIVED A DELETE CALL IN A PREVIOUS RESYNC
	case flintlocktypes.MicroVMStatus_DELETING:
		mvmScope.V(2).Info("microvm is deleting")

		return ctrl.Result{RequeueAfter: requeuePeriod}, nil
		// NO IDEA WHAT IS GOING ON WITH THIS MVM
	default:
		mvmScope.MicroVM.Status.VMState = &infrav1.VMStateUnknown
		mvmScope.SetNotReady(
			infrav1.MicrovmUnknownStateReason,
			"Error",
			errMicrovmUnknownState.Error(),
		)

		return ctrl.Result{RequeueAfter: requeuePeriod}, errMicrovmUnknownState
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MicrovmReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.Microvm{}).
		Complete(r)
}
