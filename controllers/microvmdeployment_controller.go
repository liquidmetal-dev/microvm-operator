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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1alpha1 "github.com/weaveworks-liquidmetal/microvm-operator/api/v1alpha1"
	infrav1 "github.com/weaveworks-liquidmetal/microvm-operator/api/v1alpha1"
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
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MicrovmDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.MicrovmDeployment{}).
		Owns(&infrav1.MicrovmReplicaSet{}).
		Complete(r)
}
