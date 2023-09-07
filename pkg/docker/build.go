package docker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/sieve-data/cog/pkg/util"
	"github.com/sieve-data/cog/pkg/util/console"
)

func Build(dir, dockerfile, imageUrl string, progressOutput string, writer io.Writer, imagesToPull []string) error {

	imageLatest := strings.Split(imageUrl, ":")[0] + ":latest"

	var args []string

	cache_from_images := []string{imageLatest}
	for _, image := range imagesToPull {
		cache_from_images = append(cache_from_images, image)
	}
	args = buildKitBuildArgs()
	args = append(args,
		"--file", "-",
		"--tag", imageUrl,
		"--tag", imageLatest,
		"--progress", progressOutput,
	)
	for _, image := range cache_from_images {
		args = append(args, "--cache-from", "type=registry,ref="+image)
	}

	args = append(args, ".")

	cmd := exec.Command("docker", args...)
	cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
	cmd.Dir = dir
	cmd.Stdout = writer // redirect stdout to stderr - build output is all messaging
	cmd.Stderr = writer
	cmd.Stdin = strings.NewReader(dockerfile)

	console.Debug("$ " + strings.Join(cmd.Args, " "))
	err := cmd.Run()
	if err != nil {
		return err
	}

	pushCommand := []string{"docker", "push", imageUrl}
	pushcmd := exec.Command(pushCommand[0], pushCommand[1:]...)
	pushcmd.Stdout = writer // redirect stdout to stderr - build output is all messaging
	pushcmd.Stderr = writer
	err = pushcmd.Run()
	if err != nil {
		return err
	}

	pushCommand = []string{"docker", "push", imageLatest}
	pushcmd = exec.Command(pushCommand[0], pushCommand[1:]...)
	pushcmd.Stdout = writer // redirect stdout to stderr - build output is all messaging
	pushcmd.Stderr = writer
	return pushcmd.Run()
}

func BuildAndPush(dir, dockerfile, imageUrl string, progressOutput string, writer io.Writer) error {
	err := Build(dir, dockerfile, imageUrl, progressOutput, writer, []string{})
	if err != nil {
		return err
	}
	cmd := exec.Command("docker", "push", imageUrl)
	cmd.Stdout = writer // redirect stdout to stderr - build output is all messaging
	cmd.Stderr = writer
	return cmd.Run()
}

func BuildAddLabelsToImage(image string, labels map[string]string) error {
	dockerfile := "FROM " + image
	var args []string
	if util.IsM1Mac(runtime.GOOS, runtime.GOARCH) {
		args = m1BuildxBuildArgs()
	} else {
		args = buildKitBuildArgs()
	}

	args = append(args,
		"--file", "-",
		"--tag", image,
	)
	for k, v := range labels {
		// Unlike in Dockerfiles, the value here does not need quoting -- Docker merely
		// splits on the first '=' in the argument and the rest is the label value.
		args = append(args, "--label", fmt.Sprintf(`%s=%s`, k, v))
	}
	// We're not using context, but Docker requires we pass a context
	args = append(args, ".")
	cmd := exec.Command("docker", args...)
	cmd.Stdin = strings.NewReader(dockerfile)

	console.Debug("$ " + strings.Join(cmd.Args, " "))

	if combinedOutput, err := cmd.CombinedOutput(); err != nil {
		console.Info(string(combinedOutput))
		return err
	}
	return nil
}

func m1BuildxBuildArgs() []string {
	return []string{"buildx", "build", "--platform", "linux/amd64"}
}

func buildKitBuildArgs() []string {
	return []string{"build", "--platform", "linux/amd64"}
}
