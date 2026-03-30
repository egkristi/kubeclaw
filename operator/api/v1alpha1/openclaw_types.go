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

// OpenClawStatus defines the observed state of OpenClaw
type OpenClawStatus struct {
	// Phase: Pending, Running, Failed, Terminating
	Phase string `json:"phase,omitempty"`
	
	// Conditions for the instance
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	
	// Pod name running the instance
	PodName string `json:"podName,omitempty"`
	
	// Last error message
	LastError string `json:"lastError,omitempty"`
	
	// Workspace sync status
	WorkspaceSynced bool `json:"workspaceSynced,omitempty"`
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
