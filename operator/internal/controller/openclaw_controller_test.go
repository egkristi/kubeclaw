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

package controller
/*
Copyright 2026 Erling Kristiansen.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	openclawv1alpha1 "github.com/egkristi/kubeclaw/api/v1alpha1"
)

func TestOpenClawReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = openclawv1alpha1.AddToScheme(scheme)

	tests := []struct {
		name    string
		openclaw *openclawv1alpha1.OpenClaw
		wantErr bool
	}{
		{
			name: "basic reconcile - creates resources",
			openclaw: &openclawv1alpha1.OpenClaw{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-assistant",
					Namespace: "default",
				},
				Spec: openclawv1alpha1.OpenClawSpec{
					Model: openclawv1alpha1.ModelSpec{
						Provider: "anthropic",
						Model:    "claude-sonnet-4-20250514",
					},
					Security: openclawv1alpha1.SecuritySpec{
						Sandbox: openclawv1alpha1.SandboxSpec{
							Enabled:       true,
							NetworkPolicy: true,
						},
						Egress: openclawv1alpha1.EgressSpec{
							Mode: "whitelist",
							AllowedDomains: []string{
								"api.anthropic.com",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "with workspace configuration",
			openclaw: &openclawv1alpha1.OpenClaw{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "workspace-assistant",
					Namespace: "default",
				},
				Spec: openclawv1alpha1.OpenClawSpec{
					Workspace: openclawv1alpha1.WorkspaceSpec{
						Repository: "https://github.com/example/workspace",
						Branch:     "main",
					},
					Model: openclawv1alpha1.ModelSpec{
						Provider: "openai",
						Model:    "gpt-4",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "with PVC storage",
			openclaw: &openclawv1alpha1.OpenClaw{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "persistent-assistant",
					Namespace: "default",
				},
				Spec: openclawv1alpha1.OpenClawSpec{
					Model: openclawv1alpha1.ModelSpec{
						Provider: "anthropic",
					},
					Storage: openclawv1alpha1.StorageSpec{
						Type: "PersistentVolumeClaim",
						PersistentVolumeClaim: &openclawv1alpha1.PersistentVolumeClaimSpec{
							Size:       "10Gi",
							AccessMode: "ReadWriteOnce",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.openclaw).
				Build()

			r := &OpenClawReconciler{
				Client: client,
				Scheme: scheme,
				Config: Config{
					DefaultSandboxEnabled:       true,
					DefaultSandboxNetworkPolicy: true,
					DefaultSandboxSeccomp:       true,
					DefaultEgressMode:           "whitelist",
					OpenClawImage:               "ghcr.io/openclaw/openclaw:latest",
					GitInitImage:                "alpine/git:latest",
				},
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.openclaw.Name,
					Namespace: tt.openclaw.Namespace,
				},
			}

			_, err := r.Reconcile(context.Background(), req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Reconcile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenClawReconciler_labelsForOpenClaw(t *testing.T) {
	r := &OpenClawReconciler{}
	
	openclaw := &openclawv1alpha1.OpenClaw{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-instance",
			Namespace: "default",
		},
	}

	labels := r.labelsForOpenClaw(openclaw)

	if labels["app.kubernetes.io/name"] != "openclaw" {
		t.Errorf("expected app.kubernetes.io/name=openclaw, got %s", labels["app.kubernetes.io/name"])
	}
	if labels["app.kubernetes.io/instance"] != "test-instance" {
		t.Errorf("expected app.kubernetes.io/instance=test-instance, got %s", labels["app.kubernetes.io/instance"])
	}
	if labels["app.kubernetes.io/managed-by"] != "kubeclaw-operator" {
		t.Errorf("expected app.kubernetes.io/managed-by=kubeclaw-operator, got %s", labels["app.kubernetes.io/managed-by"])
	}
}

func TestOpenClawReconciler_getAPIKeyEnvName(t *testing.T) {
	r := &OpenClawReconciler{}

	tests := []struct {
		provider string
		expected string
	}{
		{"anthropic", "ANTHROPIC_API_KEY"},
		{"openai", "OPENAI_API_KEY"},
		{"ollama", "API_KEY"},
		{"unknown", "API_KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			result := r.getAPIKeyEnvName(tt.provider)
			if result != tt.expected {
				t.Errorf("getAPIKeyEnvName(%s) = %s, expected %s", tt.provider, result, tt.expected)
			}
		})
	}
}

func TestOpenClawReconciler_shouldCreateNetworkPolicy(t *testing.T) {
	tests := []struct {
		name     string
		openclaw *openclawv1alpha1.OpenClaw
		config   Config
		expected bool
	}{
		{
			name: "enabled in spec",
			openclaw: &openclawv1alpha1.OpenClaw{
				Spec: openclawv1alpha1.OpenClawSpec{
					Security: openclawv1alpha1.SecuritySpec{
						Sandbox: openclawv1alpha1.SandboxSpec{
							NetworkPolicy: true,
						},
					},
				},
			},
			config:   Config{DefaultSandboxNetworkPolicy: false},
			expected: true,
		},
		{
			name: "enabled by default",
			openclaw: &openclawv1alpha1.OpenClaw{
				Spec: openclawv1alpha1.OpenClawSpec{},
			},
			config:   Config{DefaultSandboxNetworkPolicy: true},
			expected: true,
		},
		{
			name: "disabled",
			openclaw: &openclawv1alpha1.OpenClaw{
				Spec: openclawv1alpha1.OpenClawSpec{
					Security: openclawv1alpha1.SecuritySpec{
						Sandbox: openclawv1alpha1.SandboxSpec{
							NetworkPolicy: false,
						},
					},
				},
			},
			config:   Config{DefaultSandboxNetworkPolicy: false},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &OpenClawReconciler{Config: tt.config}
			result := r.shouldCreateNetworkPolicy(tt.openclaw)
			if result != tt.expected {
				t.Errorf("shouldCreateNetworkPolicy() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
