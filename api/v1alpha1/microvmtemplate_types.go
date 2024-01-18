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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MicrovmTemplateSpec defines the desired state of MicrovmTemplate
type MicrovmTemplateSpec struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the Microvm.
	// +optional
	Spec MicrovmSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MicrovmTemplate is the Schema for the microvmtemplates API
type MicrovmTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Template defines the Microvm that will be created from this pod template.
	// +optional
	Template MicrovmTemplateSpec `json:"template,omitempty"`
}

//+kubebuilder:object:root=true

// MicrovmTemplateList contains a list of MicrovmTemplate
type MicrovmTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MicrovmTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MicrovmTemplate{}, &MicrovmTemplateList{})
}
