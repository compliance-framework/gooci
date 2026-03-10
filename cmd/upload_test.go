package cmd

import (
	"os"
	"path"
	"testing"
)

var (
	validDirectory   = "."
	validRespository = "ghcr.io/compliance-framework/plugin-local-ssh:v1.0.0"
)

func Test_UploadCmd_ValidateArgs(t *testing.T) {
	uploadCmd := uploadRelease{}

	tests := []struct {
		name       string
		args       []string
		shouldFail bool
	}{
		{
			name: "Directory and repository are valid",
			args: []string{
				validDirectory,
				validRespository,
			},
			shouldFail: false,
		},
		{
			name: "Directory doesn't exist",
			args: []string{
				"./some-non-existant-directory",
				validRespository,
			},
			shouldFail: true,
		},
		{
			name: "Repository is not valid",
			args: []string{
				validDirectory,
				"invalid/invalid",
			},
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uploadCmd.validateArgs(tt.args)
			if (err != nil) != tt.shouldFail {
				t.Errorf("uploadCmd.validateArgs() error = %v, shouldFail %v", err, tt.shouldFail)
			}
		})
	}
}

func Test_UploadCmd_ValidateArgs_Response(t *testing.T) {
	t.Run("Returns correct Config", func(t *testing.T) {
		uploadCmd := uploadRelease{}

		config, err := uploadCmd.validateArgs([]string{validDirectory, validRespository})
		if err != nil {
			t.Errorf("uploadCmd.validateArgs() error = %v", err)
		}

		if config == nil {
			t.Errorf("uploadCmd.validateArgs() config = %v", config)
		}

		if config.tag.String() != validRespository {
			t.Errorf("uploadCmd.validateArgs() config = %v", config)
		}

		workDir, err := os.Getwd()
		if err != nil {
			t.Errorf("uploadCmd.validateArgs() error = %v", err)
		}
		if config.source != path.Join(workDir, validDirectory) {
			t.Errorf("uploadCmd.validateArgs() config = %v", config)
		}
	})
}

func Test_mergeAnnotations(t *testing.T) {
	defaults := map[string]string{
		"org.opencontainers.image.title":    "default-title",
		"org.opencontainers.image.ref.name": "v1.0.0",
	}
	passed := map[string]string{
		"org.opencontainers.image.title": "override-title",
		"custom.annotation":              "custom-value",
	}

	merged := mergeAnnotations(defaults, passed)

	if got := merged["org.opencontainers.image.title"]; got != "override-title" {
		t.Fatalf("expected passed annotation to override default, got %q", got)
	}

	if got := merged["org.opencontainers.image.ref.name"]; got != "v1.0.0" {
		t.Fatalf("expected default annotation to remain, got %q", got)
	}

	if got := merged["custom.annotation"]; got != "custom-value" {
		t.Fatalf("expected custom passed annotation to be present, got %q", got)
	}

	if len(merged) != 3 {
		t.Fatalf("expected 3 merged annotations, got %d", len(merged))
	}
}
