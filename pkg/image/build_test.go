package image

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sieve-data/cog/pkg/config"
)

func TestGenerate(t *testing.T) {
	config := &config.Config{
		Build: &config.Build{
			GPU:           true,
			PythonVersion: "3.10",
			PythonPackages: []string{
				"torch",
				"ffmpeg-python==0.2.0",
			},
			CUDA: "12.1",
		},
	}

	tmpDir := t.TempDir()
	barFilePath := filepath.Join(tmpDir, "bar.txt")
	err := os.WriteFile(barFilePath, []byte(""), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	dockerfileContents, err := Build(config, tmpDir, "test", "progress", os.Stdout, []string{})
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Generated Dockerfile content:")
	t.Log(dockerfileContents)
}
