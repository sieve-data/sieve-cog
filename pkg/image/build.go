package image

import (
	"fmt"
	"io"
	"os"

	"github.com/sieve-data/cog/pkg/config"
	"github.com/sieve-data/cog/pkg/docker"
	"github.com/sieve-data/cog/pkg/dockerfile"
	"github.com/sieve-data/cog/pkg/util/console"
)

// Build a Cog model from a config
//
// This is separated out from docker.Build(), so that can be as close as possible to the behavior of 'docker build'.
func Build(cfg *config.Config, dir, imageName string, progressOutput string, writer io.Writer, imagesToPull []string) (string, error) {
	console.Info(fmt.Sprint("cudav version before validate and complete", cfg.Build.CUDA))
	cfg.ValidateAndCompleteCUDA()
	console.Info(fmt.Sprint("cudav after before validate and complete", cfg.Build.CUDA))
	console.Infof("Building Docker image from environment in cog.yaml as %s...", imageName)

	generator, err := dockerfile.NewGenerator(cfg, dir)
	if err != nil {
		return "", fmt.Errorf("Error creating Dockerfile generator: %w", err)
	}
	defer func() {
		if err := generator.Cleanup(); err != nil {
			console.Warnf("Error cleaning up Dockerfile generator: %s", err)
		}
	}()

	dockerfileContents, err := generator.Generate()
	if err != nil {
		return "", fmt.Errorf("Failed to generate Dockerfile: %w", err)
	}

	if err := docker.Build(dir, dockerfileContents, imageName, progressOutput, writer, imagesToPull); err != nil {
		return "", fmt.Errorf("Failed to build Docker image: %w", err)
	}

	console.Info("Adding labels to image...")
	return dockerfileContents, nil
}

func Generate(cfg *config.Config, dir string) (string, string, error) {
	console.Info(fmt.Sprint("cudav version before validate and complete", cfg.Build.CUDA))
	cfg.ValidateAndCompleteCUDA()
	console.Info(fmt.Sprint("cudav after before validate and complete", cfg.Build.CUDA))

	generator, err := dockerfile.NewGenerator(cfg, dir)
	if err != nil {
		return "", "", fmt.Errorf("Error creating Dockerfile generator: %w", err)
	}
	defer func() {
		if err := generator.Cleanup(); err != nil {
			console.Warnf("Error cleaning up Dockerfile generator: %s", err)
		}
	}()

	dockerfileContents, err := generator.Generate()
	if err != nil {
		return "", "", fmt.Errorf("Failed to generate Dockerfile: %w", err)
	}

	cogSHA256 := generator.CogSHA256()

	return dockerfileContents, cogSHA256, nil
}

func BuildFromGenerate(cfg *config.Config, dir, imageName string, progressOutput string, writer io.Writer, dockerfileContents string) (string, error) {
	console.Infof("Building Docker image from environment in cog.yaml as %s...", imageName)

	imagesToPull := []string{}

	if err := docker.Build(dir, dockerfileContents, imageName, progressOutput, writer, imagesToPull); err != nil {
		return "", fmt.Errorf("Failed to build Docker image: %w", err)
	}

	console.Info("Adding labels to image...")

	return dockerfileContents, nil
}

func BuildBase(cfg *config.Config, dir string, progressOutput string) (string, error) {
	// TODO: better image management so we don't eat up disk space
	// https://github.com/sieve-data/cog/issues/80
	imageName := config.BaseDockerImageName(dir)
	imagesToPull := []string{}

	console.Info("Building Docker image from environment in cog.yaml...")
	generator, err := dockerfile.NewGenerator(cfg, dir)
	if err != nil {
		return "", fmt.Errorf("Error creating Dockerfile generator: %w", err)
	}
	defer func() {
		if err := generator.Cleanup(); err != nil {
			console.Warnf("Error cleaning up Dockerfile generator: %s", err)
		}
	}()
	dockerfileContents, err := generator.GenerateBase()
	if err != nil {
		return "", fmt.Errorf("Failed to generate Dockerfile: %w", err)
	}
	if err := docker.Build(dir, dockerfileContents, imageName, progressOutput, os.Stderr, imagesToPull); err != nil {
		return "", fmt.Errorf("Failed to build Docker image: %w", err)
	}
	return imageName, nil
}
