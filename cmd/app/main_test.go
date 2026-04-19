package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Gabrielbdd/gospa/config"
)

func TestLoadProvisionerPAT(t *testing.T) {
	dir := t.TempDir()

	goodFile := filepath.Join(dir, "good.pat")
	if err := os.WriteFile(goodFile, []byte("token-from-config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(dir, "env.pat")
	if err := os.WriteFile(envFile, []byte("token-from-env\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	emptyFile := filepath.Join(dir, "empty.pat")
	if err := os.WriteFile(emptyFile, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		configPath string
		envPath   string
		want      string
		wantErr   string
	}{
		{
			name:       "reads config path when env is empty",
			configPath: goodFile,
			want:       "token-from-config",
		},
		{
			name:       "env path overrides config path",
			configPath: goodFile,
			envPath:    envFile,
			want:       "token-from-env",
		},
		{
			name:    "empty config + no env errors with actionable message",
			wantErr: "no provisioner PAT path configured",
		},
		{
			name:       "missing file produces actionable error",
			configPath: filepath.Join(dir, "nonexistent.pat"),
			wantErr:    "reading provisioner PAT",
		},
		{
			name:       "empty file produces error",
			configPath: emptyFile,
			wantErr:    "reading provisioner PAT",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(provisionerPATEnv, tc.envPath)

			cfg := &config.Config{
				Zitadel: config.ZitadelConfig{ProvisionerPatFile: tc.configPath},
			}

			got, err := loadProvisionerPAT(cfg)
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
