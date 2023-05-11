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

func Build(dir, dockerfile, imageUrl string, progressOutput string, writer io.Writer) error {

	// write dockerfile to dir
	// err := os.WriteFile(dir+"/Dockerfile", []byte(dockerfile), 0644)
	// if err != nil {
	// 	return err
	// }

	imageLatest := strings.Split(imageUrl, ":")[0] + ":latest"

	// cloudbuildYaml := fmt.Sprintf(
	// `steps:
	//  - name: 'ghcr.io/depot/cli:latest'
	//    env:
	//     - DEPOT_TOKEN=%s
	//    args: ['build', '--project', 'zz1b68kjbv', '-t', '%s', '.', "--push"]
	// `, depotToken, imageUrl)

	// tmpFile, err := ioutil.TempFile(os.TempDir(), "cloudbuild.yaml")
	// if err != nil {
	// 	return err
	// }
	// defer os.Remove(tmpFile.Name())
	// tmpFile.Write([]byte(cloudbuildYaml))

	// // close the file
	// if err := tmpFile.Close(); err != nil {
	// 	return err
	// }

	// pullCommand := []string{"docker", "pull", imageLatest}
	// pullcmd := exec.Command(pullCommand[0], pullCommand[1:]...)
	// pullcmd.Stdout = writer // redirect stdout to stderr - build output is all messaging
	// pullcmd.Stderr = writer
	// pullcmd.Run()

	var args []string
	// if util.IsM1Mac(runtime.GOOS, runtime.GOARCH) {
	// 	args = m1BuildxBuildArgs()
	// } else {
	// 	args = buildKitBuildArgs()
	// }
	cache_from_images := []string{
		imageLatest, 
		"us-central1-docker.pkg.dev/sieve-grapefruit/grapefruit-containers/base-images/cuda-11-2:latest",
		"us-central1-docker.pkg.dev/sieve-grapefruit/grapefruit-containers/base-images/cuda-11-8:latest",
		"us-central1-docker.pkg.dev/sieve-grapefruit/grapefruit-containers/base-images/ffmpeg-python:latest",
		"us-central1-docker.pkg.dev/sieve-grapefruit/grapefruit-containers/base-images/basic-python:latest",
	}
	args = buildKitBuildArgs() //[]string{"buildx", "--project", depotProjectId, "-t", imageUrl, ".", "--push"}
	args = append(args,
		// "--load",
		// "--build-arg", "BUILDKIT_INLINE_CACHE=1",
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
	err := Build(dir, dockerfile, imageUrl, progressOutput, writer)
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
