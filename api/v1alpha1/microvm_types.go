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
	flclient "github.com/weaveworks-liquidmetal/controller-pkg/client"
	microvm "github.com/weaveworks-liquidmetal/controller-pkg/types/microvm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// MvmFinalizer allows ReconcileMicrovm to clean up resources associated with Microvm
	// before removing it from the apiserver.
	MvmFinalizer = "microvm.infrastructure.microvm.x-k8s.io"
)

// MicrovmSpec defines the desired state of Microvm
type MicrovmSpec struct {
	// Host sets the host device address for Microvm creation.
	// +kubebuilder:validation:Required
	Host microvm.Host `json:"host"`
	// MicrovmProxy is the proxy server details to use when calling the microvm service. This is an
	// alternative to using the http proxy environment variables and applied purely to the grpc service.
	MicrovmProxy *flclient.Proxy `json:"microvmProxy,omitempty"`
	// VMSpec contains the Microvm spec.
	// +kubebuilder:validation:Required
	microvm.VMSpec `json:",inline"`
	// UserData is additional userdata script to execute in the Microvm's cloud init.
	// This can be in the form of a raw shell script, eg:
	// userdata: |
	//   #!/bin/bash
	//   echo "hi from my microvm"
	//
	// or in valid cloud-config, eg:
	// userdata: |
	// 	#cloud-config
	// 	write_files:
	// 	- content: "hello"
	// 		path: "/root/FINDME"
	// 		owner: "root:root"
	// 		permissions: "0755"
	// +optional
	UserData *string `json:"userdata"`
	// SSHPublicKeys is list of SSH public keys which will be added to the Microvm.
	// +optional
	SSHPublicKeys []microvm.SSHPublicKey `json:"sshPublicKeys,omitempty"`
	// mTLS Configuration:
	//
	// It is recommended that each flintlock host is configured with its own cert
	// signed by a common CA, and set to use mTLS.
	// The flintlock-operator should be provided with the CA, and a client cert and key
	// signed by that CA.
	// TLSSecretRef is a reference to the name of a secret which contains TLS cert information
	// for connecting to Flintlock hosts.
	// The secret should be created in the same namespace as the MicroVMCluster.
	// The secret should be of type Opaque
	// with the addition of a ca.crt key.
	//
	// apiVersion: v1
	// kind: Secret
	// metadata:
	// 	name: secret-tls
	// 	namespace: default  <- same as Cluster
	// type: Opaque
	// data:
	// 	tls.crt: |
	// 		-----BEGIN CERTIFICATE-----
	// 		MIIC2DCCAcCgAwIBAgIBATANBgkqh ...
	// 		-----END CERTIFICATE-----
	// 	tls.key: |
	// 		-----BEGIN EC PRIVATE KEY-----
	// 		MIIEpgIBAAKCAQEA7yn3bRHQ5FHMQ ...
	// 		-----END EC PRIVATE KEY-----
	// 	ca.crt: |
	// 		-----BEGIN CERTIFICATE-----
	// 		MIIEpgIBAAKCAQEA7yn3bRHQ5FHMQ ...
	// 		-----END CERTIFICATE-----
	// +optional
	TLSSecretRef string `json:"tlsSecretRef,omitempty"`
	// ProviderID is the unique identifier as specified by the cloud provider.
	ProviderID *string `json:"providerID,omitempty"`
}

// MicrovmStatus defines the observed state of Microvm
type MicrovmStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// VMState indicates the state of the microvm.
	VMState *microvm.VMState `json:"vmState,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`
	// Conditions defines current service state of the Microvm.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Microvm is the Schema for the microvms API
type Microvm struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MicrovmSpec   `json:"spec,omitempty"`
	Status MicrovmStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MicrovmList contains a list of Microvm
type MicrovmList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Microvm `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Microvm{}, &MicrovmList{})
}

// GetConditions returns the observations of the operational state of the MicrovmMachine resource.
func (r *Microvm) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the MicrovmMachine to the predescribed clusterv1.Conditions.
func (r *Microvm) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}
