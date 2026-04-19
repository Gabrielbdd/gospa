package main

import (
	"strings"
	"testing"

	"github.com/Gabrielbdd/gospa/config"
)

// TestResolveProvisionerPATPath covers the precedence between the
// env-var override (Kubernetes Secret mount path) and gofra.yaml's
// zitadel.provisioner_pat_file. The actual file read + last-known-good
// behavior live in internal/patwatch and are tested there.
func TestResolveProvisionerPATPath(t *testing.T) {
	tests := []struct {
		name       string
		configPath string
		envPath    string
		want       string
		wantErr    string
	}{
		{
			name:       "env wins over config when both are set",
			configPath: "/from/config.pat",
			envPath:    "/from/env.pat",
			want:       "/from/env.pat",
		},
		{
			name:       "config used when env is empty",
			configPath: "/from/config.pat",
			want:       "/from/config.pat",
		},
		{
			name:    "neither set returns actionable error",
			wantErr: "no provisioner PAT path configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(provisionerPATEnv, tc.envPath)
			cfg := &config.Config{
				Zitadel: config.ZitadelConfig{ProvisionerPatFile: tc.configPath},
			}

			got, err := resolveProvisionerPATPath(cfg)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %v; want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q; want %q", got, tc.want)
			}
		})
	}
}
