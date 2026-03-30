/*
Copyright 2026 Erling Kristiansen.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenClawSpec defines the desired state of OpenClaw
type OpenClawSpec struct {
	// Workspace configuration for the OpenClaw instance
	Workspace WorkspaceSpec `json:"workspace,omitempty"`

	// Model configuration for inference
	Model ModelSpec `json:"model,omitempty"`

	// Security policies for the instance
	Security SecuritySpec `json:"security,omitempty"`

	// Resource requirements
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Channel configurations
	Channels ChannelsSpec `json:"channels,omitempty"`

	// Storage configuration for ~/.openclaw
	// If not specified, uses emptyDir (ephemeral)
	Storage StorageSpec `json:"storage,omitempty"`
}

// WorkspaceSpec defines the workspace configuration
type WorkspaceSpec struct {
	// Git repository URL for the workspace
	Repository string `json:"repository,omitempty"`

	// Git branch to clone (default: main)
	Branch string `json:"branch,omitempty"`

	// Credentials for private repositories
	Credentials SecretRef `json:"credentials,omitempty"`
}

// SecretRef references a Kubernetes secret
type SecretRef struct {
	// Name of the secret
	Name string `json:"name,omitempty"`

	// Namespace of the secret (defaults to same namespace as OpenClaw)
	Namespace string `json:"namespace,omitempty"`
}

// StorageSpec defines storage options for ~/.openclaw
type StorageSpec struct {
	// Type of storage: EmptyDir (default) or PersistentVolumeClaim
	// +kubebuilder:validation:Enum=EmptyDir;PersistentVolumeClaim
	Type string `json:"type,omitempty"`

	// PersistentVolumeClaim configuration
	// Required when type is PersistentVolumeClaim
	PersistentVolumeClaim *PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`

	// EmptyDir configuration
	// Optional when type is EmptyDir
	EmptyDir *EmptyDirSpec `json:"emptyDir,omitempty"`
}

// PersistentVolumeClaimSpec defines PVC configuration
type PersistentVolumeClaimSpec struct {
	// Storage class name (optional, uses default if not specified)
	StorageClassName *string `json:"storageClassName,omitempty"`

	// Access mode (default: ReadWriteOnce)
	// +kubebuilder:validation:Enum=ReadWriteOnce;ReadOnlyMany;ReadWriteMany
	AccessMode string `json:"accessMode,omitempty"`

	// Storage size (e.g., "10Gi", "20Gi")
	// +kubebuilder:validation:Pattern=^[0-9]+[GM]i$
	Size string `json:"size,omitempty"`

	// Volume mode (default: Filesystem)
	// +kubebuilder:validation:Enum=Filesystem;Block
	VolumeMode string `json:"volumeMode,omitempty"`

	// Selector for matching existing PVs
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// EmptyDirSpec defines EmptyDir configuration
type EmptyDirSpec struct {
	// Medium type: Memory or empty (disk)
	// +kubebuilder:validation:Enum=Memory;""
	Medium string `json:"medium,omitempty"`

	// Size limit (e.g., "10Gi")
	// +optional
	SizeLimit *string `json:"sizeLimit,omitempty"`
}

// ModelSpec defines the AI model configuration
type ModelSpec struct {
	// Provider: anthropic, openai, ollama
	Provider string `json:"provider,omitempty"`

	// Secret containing the API key
	APIKeySecretRef string `json:"apiKeySecretRef,omitempty"`

	// Model identifier
	Model string `json:"model,omitempty"`

	// Base URL for custom endpoints (optional)
	BaseURL string `json:"baseURL,omitempty"`
}

// SecuritySpec defines security policies
type SecuritySpec struct {
	// Sandbox configuration
	Sandbox SandboxSpec `json:"sandbox,omitempty"`

	// Egress policy
	Egress EgressSpec `json:"egress,omitempty"`
}

// SandboxSpec defines sandboxing options
type SandboxSpec struct {
	// Enable sandboxing
	Enabled bool `json:"enabled,omitempty"`

	// Enable Landlock filesystem sandboxing
	Landlock bool `json:"landlock,omitempty"`

	// Enable seccomp
	Seccomp bool `json:"seccomp,omitempty"`

	// Enable network policy
	NetworkPolicy bool `json:"networkPolicy,omitempty"`
}

// EgressSpec defines network egress policies
type EgressSpec struct {
	// Mode: whitelist, blacklist, deny-all
	Mode string `json:"mode,omitempty"`

	// Allowed domains for whitelist mode
	AllowedDomains []string `json:"allowedDomains,omitempty"`

	// Blocked domains for blacklist mode
	BlockedDomains []string `json:"blockedDomains,omitempty"`
}

// ChannelsSpec defines messaging channels
type ChannelsSpec struct {
	// Telegram configuration
	Telegram TelegramSpec `json:"telegram,omitempty"`

	// Email configuration
	Email EmailSpec `json:"email,omitempty"`

	// Webhook configuration
	Webhook WebhookSpec `json:"webhook,omitempty"`
}

// TelegramSpec defines Telegram bot configuration
type TelegramSpec struct {
	// Enable Telegram channel
	Enabled bool `json:"enabled,omitempty"`

	// Secret containing bot token
	TokenSecretRef string `json:"tokenSecretRef,omitempty"`
}

// EmailSpec defines email configuration
type EmailSpec struct {
	// Enable email channel
	Enabled bool `json:"enabled,omitempty"`

	// Secret containing SMTP credentials
	SMTPSecretRef string `json:"smtpSecretRef,omitempty"`
}

// WebhookSpec defines webhook configuration
type WebhookSpec struct {
	// Enable webhook channel
	Enabled bool `json:"enabled,omitempty"`

	// Port for webhook server
	Port int32 `json:"port,omitempty"`
}

// Condition contains details for one aspect of the current state
type Condition struct {
	// Type of condition
	Type string `json:"type"`

	// Status of the condition (True, False, Unknown)
	Status string `json:"status"`

	// Reason for the condition's last transition
	Reason string `json:"reason,omitempty"`

	// Message with details about the transition
	Message string `json:"message,omitempty"`

	// LastTransitionTime is the last time the condition transitioned
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// OpenClawStatus defines the observed state of OpenClaw
type OpenClawStatus struct {
	// Phase: Pending, Running, Failed, Terminating
	Phase string `json:"phase,omitempty"`

	// Conditions for the instance
	Conditions []Condition `json:"conditions,omitempty"`

	// Pod name running the instance
	PodName string `json:"podName,omitempty"`

	// Last error message
	LastError string `json:"lastError,omitempty"`

	// Workspace sync status
	WorkspaceSynced bool `json:"workspaceSynced,omitempty"`

	// Storage status
	StorageReady bool `json:"storageReady,omitempty"`

	// PersistentVolumeClaim name (if using PVC)
	PVCName string `json:"pvcName,omitempty"`

	// ServiceName is the name of the Service for this instance
	ServiceName string `json:"serviceName,omitempty"`

	// NetworkPolicyName is the name of the NetworkPolicy
	NetworkPolicyName string `json:"networkPolicyName,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=oc

// OpenClaw is the Schema for the openclaws API
type OpenClaw struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenClawSpec   `json:"spec,omitempty"`
	Status OpenClawStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenClawList contains a list of OpenClaw
type OpenClawList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenClaw `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenClaw{}, &OpenClawList{})
}
