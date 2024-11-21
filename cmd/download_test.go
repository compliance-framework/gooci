package cmd

import (
	"os"
	"path"
	"testing"
)

func Test_DownloadCmd_ValidateArgs(t *testing.T) {
	downloadCmd := downloadRelease{}

	tests := []struct {
		name       string
		args       []string
		shouldFail bool
	}{
		{
			name: "Directory and repository are valid",
			args: []string{
				validRespository,
				validDirectory,
			},
			shouldFail: false,
		},
		{
			name: "Repository is not valid",
			args: []string{
				"invalid/invalid",
				validDirectory,
			},
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := downloadCmd.validateArgs(tt.args)
			if (err != nil) != tt.shouldFail {
				t.Errorf("downloadCmd.validateArgs() error = %v, shouldFail %v", err, tt.shouldFail)
			}
		})
	}
}

func Test_DownloadCmd_ValidateArgs_Response(t *testing.T) {
	t.Run("Returns correct Config", func(t *testing.T) {
		downloadCmd := downloadRelease{}

		config, err := downloadCmd.validateArgs([]string{validRespository, validDirectory})
		if err != nil {
			t.Errorf("downloadCmd.validateArgs() error = %v", err)
		}

		if config == nil {
			t.Errorf("downloadCmd.validateArgs() config = %v", config)
		}

		if config.source.String() != validRespository {
			t.Errorf("downloadCmd.validateArgs() config = %v", config)
		}

		workDir, err := os.Getwd()
		if err != nil {
			t.Errorf("downloadCmd.validateArgs() error = %v", err)
		}
		if config.directory != path.Join(workDir, validDirectory) {
			t.Errorf("downloadCmd.validateArgs() config = %v", config)
		}
	})
}
