package dockerfile

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

	g, err := NewGenerator(config, tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	str, err := g.Generate()

	if err != nil {
		t.Fatal(err)
	}

	t.Log("Generated Dockerfile content:")
	t.Log(str)
}
