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
	err := os.WriteFile(dir+"/Dockerfile", []byte(dockerfile), 0644)
	if err != nil {
		return err
	}

	// imageLatest := strings.Split(imageUrl, ":")[0] + ":latest"

	depotToken := "depot_project_690526a1d60f58cf2a9563a9faa6fd32319f42997bcc389dea0a7585bbfc01d8"
	depotProjectId := "zz1b68kjbv"

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

	var args []string
	// if util.IsM1Mac(runtime.GOOS, runtime.GOARCH) {
	// 	args = m1BuildxBuildArgs()
	// } else {
	// 	args = buildKitBuildArgs()
	// }
	args = []string{"build", "--project", depotProjectId, "-t", imageUrl, ".", "--push"}
	// args = append(args,
	// 	// "--file", "-",
	// 	// "--build-arg", "BUILDKIT_INLINE_CACHE=1",
	// 	// "--tag", imageName,
	// 	// "--progress", progressOutput,
	// 	// ".",
	// )
	cmd := exec.Command("depot", args...)
	cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1", "DEPOT_TOKEN="+depotToken)
	cmd.Dir = dir
	cmd.Stdout = writer // redirect stdout to stderr - build output is all messaging
	cmd.Stderr = writer

	console.Debug("$ " + strings.Join(cmd.Args, " "))
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
	return []string{"buildx", "build", "--platform", "linux/amd64", "--load"}
}

func buildKitBuildArgs() []string {
	return []string{"buildx", "build", "--platform", "linux/amd64", "--load", "--cache-to", "type=registry,ref=us-central1-docker.pkg.dev/sieve-data/cog/cog-cache:latest"}
}
