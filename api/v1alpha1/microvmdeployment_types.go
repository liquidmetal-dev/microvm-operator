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

package v1alpha1

import (
	microvm "github.com/weaveworks-liquidmetal/controller-pkg/types/microvm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// MvDeploymentSFinalizer allows ReconcileMicrovmDeployment to clean up resources associated with the Deployment
	// before removing it from the apiserver.
	MvmDeploymentFinalizer = "microvmdeployment.infrastructure.microvm.x-k8s.io"
)

type HostMap map[string]struct{}

// MicrovmDeploymentSpec defines the desired state of MicrovmDeployment
type MicrovmDeploymentSpec struct {
	// Replicas is the number of Microvms to create on the given Host with the given
	// Microvm spec
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`
	// Host sets the host device address for Microvm creation.
	// +kubebuilder:validation:Required
	Hosts []microvm.Host `json:"hosts,omitempty"`
	// Template is the object that describes the Microvm that will be created if
	// insufficient replicas are detected.
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller#pod-template
	// +optional
	Template MicrovmTemplateSpec `json:"template,omitempty" protobuf:"bytes,3,opt,name=template"`
}

// MicrovmDeploymentStatus defines the observed state of MicrovmDeployment
type MicrovmDeploymentStatus struct {
	// Ready is true when all Replicas report ready
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// Replicas is the most recently observed number of replicas which have been created.
	// +optional
	Replicas int32 `json:"replicas"`

	// ReadyReplicas is the number of microvms controlled by this Deployment with a Ready Condition.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Represents the latest available observations of a deployments's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MicrovmDeployment is the Schema for the microvmdeployments API
type MicrovmDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MicrovmDeploymentSpec   `json:"spec,omitempty"`
	Status MicrovmDeploymentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MicrovmDeploymentList contains a list of MicrovmDeployment
type MicrovmDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MicrovmDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MicrovmDeployment{}, &MicrovmDeploymentList{})
}

// GetConditions returns the observations of the operational state of the MicrovmMachine resource.
func (r *MicrovmDeployment) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the MicrovmMachine to the predescribed clusterv1.Conditions.
func (r *MicrovmDeployment) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}
