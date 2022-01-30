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

package v1

import (
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NamespaceRef contains the namespace
type NamespaceRef string

// SyncPhase refers the phase
// +kubebuilder:validation:Enum=Provisioning;Syncing;NotSyncing;
type SyncPhase string

// SecretSyncSpec defines the desired state of SecretSync
type SecretSyncSpec struct {
	// SourceRef is the name of source secret to be synced
	SourceRef core.SecretReference `json:"sourceRef"`

	// TargetNamespaces is an array of namespaces where the secrets will be synced
	TargetNamespaces []NamespaceRef `json:"targetNamespaces"`

	// This flag tells the controller to pause the sync.
	// Defaults to false.
	// +optional
	Paused *bool `json:"pause,omitempty"`
}

// SecretSyncStatus defines the observed state of SecretSync
type SecretSyncStatus struct {
	/// Specifies the current phase of the secret sync
	// +optional
	Phase SyncPhase `json:"phase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SecretSync is the Schema for the secretsyncs API
type SecretSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretSyncSpec   `json:"spec,omitempty"`
	Status SecretSyncStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SecretSyncList contains a list of SecretSync
type SecretSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecretSync{}, &SecretSyncList{})
}
