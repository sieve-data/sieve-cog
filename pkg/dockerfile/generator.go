package dockerfile

import (
	// blank import for embeds
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sieve-data/cog/pkg/config"
)

//go:embed embed/cog.whl
var cogWheelEmbed []byte

var SieveRequirements = []string{
	"requests",
	"click",
	"pydantic",
	"pathlib",
	"typing",
	"argparse",
	"tqdm",
	"uuid",
	"networkx",
	"typeguard",
	"pillow",
	"typer",
	"rich",
	"cloudpickle",
	"docstring_parser",
	"jsonref",
	"protobuf",
	"pyyaml",
	"grpcio",
}

type Generator struct {
	Config *config.Config
	Dir    string

	// these are here to make this type testable
	GOOS   string
	GOARCH string

	// absolute path to tmpDir, a directory that will be cleaned up
	tmpDir string
	// tmpDir relative to Dir
	relativeTmpDir string
}

func NewGenerator(config *config.Config, dir string) (*Generator, error) {
	rootTmp := path.Join(dir, ".cog/tmp/build")
	if err := os.MkdirAll(rootTmp, 0o755); err != nil {
		return nil, err
	}
	// tmpDir, but without dir prefix. This is the path used in the Dockerfile.
	relativeTmpDir, err := filepath.Rel(dir, rootTmp)
	if err != nil {
		return nil, err
	}

	return &Generator{
		Config:         config,
		Dir:            dir,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOOS,
		tmpDir:         rootTmp,
		relativeTmpDir: relativeTmpDir,
	}, nil
}

func (g *Generator) GenerateBase() (string, error) {
	baseImage, err := g.baseImage()
	if err != nil {
		return "", err
	}

	installPython := ""
	if g.Config.Build.GPU {
		installPython, err = g.installPythonCUDA()
		if err != nil {
			return "", err
		}
	}

	aptInstalls, err := g.aptInstalls()
	if err != nil {
		return "", err
	}

	pythonRequirements, err := g.pythonRequirements()
	if err != nil {
		return "", err
	}

	pipInstalls, err := g.pipInstalls()
	if err != nil {
		return "", err
	}

	run, err := g.run()
	if err != nil {
		return "", err
	}

	return strings.Join(filterEmpty([]string{
		"# syntax = docker/dockerfile:1.2",
		"FROM " + baseImage,
		g.preamble(),
		g.installTini(),
		installPython,
		g.installCython(),
		g.sieveRequirements(),
		aptInstalls,
		pipInstalls,
		pythonRequirements,
		g.setupNetworking(),
		run,
		`WORKDIR /src`,
		`EXPOSE 5000`,
		`CMD ["python", "-m", "cog.server.http"]`,
	}), "\n"), nil
}

func (g *Generator) Generate() (string, error) {
	base, err := g.GenerateBase()
	if err != nil {
		return "", err
	}
	return strings.Join(filterEmpty([]string{
		base,
		// `COPY . /src`,
	}), "\n"), nil
}

func (g *Generator) Cleanup() error {
	if err := os.RemoveAll(g.tmpDir); err != nil {
		return fmt.Errorf("Failed to clean up %s: %w", g.tmpDir, err)
	}
	return nil
}

func (g *Generator) baseImage() (string, error) {
	if g.Config.Build.GPU {
		return g.Config.CUDABaseImageTag()
	}
	return "python:" + g.Config.Build.PythonVersion, nil
}

func (g *Generator) preamble() string {
	return `ENV DEBIAN_FRONTEND=noninteractive
ENV PYTHONUNBUFFERED=1
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/lib/x86_64-linux-gnu:/usr/local/nvidia/lib64:/usr/local/nvidia/bin`
}

func (g *Generator) installTini() string {
	// Install tini as the image entrypoint to provide signal handling and process
	// reaping appropriate for PID 1.
	//
	// N.B. If you remove/change this, consider removing/changing the `has_init`
	// image label applied in image/build.go.
	lines := []string{
		`RUN --mount=type=cache,target=/var/cache/apt set -eux; \
apt-get update -qq; \
apt-get install -qqy --no-install-recommends curl; \
rm -rf /var/lib/apt/lists/*; \
TINI_VERSION=v0.19.0; \
TINI_ARCH="$(dpkg --print-architecture)"; \
curl -sSL -o /sbin/tini "https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-${TINI_ARCH}"; \
chmod +x /sbin/tini`,
		`ENTRYPOINT ["/sbin/tini", "--", "ip", "netns", "exec", "worker"]`,
	}
	return strings.Join(lines, "\n")
}

func (g *Generator) setupNetworking() string {
	var portsArray []string
	for i := 0; i < 16; i++ { // 2 ports per container, one for health check and one for the actual grpc prediction server. 8 max containers -> 16 ports to open
		portsArray = append(portsArray, fmt.Sprintf("%d", i + 50054))
	}
	line := `RUN ip netns add worker; \
ip link add veth1 type veth peer name veth2; \
ip link set veth2 netns worker; \
ip addr add 10.0.0.1/24 dev veth1; \
ip link set veth1 up; \
ip netns exec mynetns ip addr add 10.0.0.2/24 dev veth2; \
ip netns exec mynetns ip link set veth2 up; \
ip netns exec mynetns ip link set lo up; \
sysctl -w net.ipv4.ip_forward=1; \
iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE; \
mkdir -p /etc/netns/worker; \
cp /etc/resolv.conf /etc/netns/worker/resolv.conf; \
ip netns exec worker iptables -A INPUT -p tcp -m multiport --dports ` + strings.Join(portsArray, ",") + ` -j ACCEPT; \
ip netns exec worker iptables -A INPUT -p tcp -j DROP`
	return line
}

func (g *Generator) aptInstalls() (string, error) {
	packages := g.Config.Build.SystemPackages
	if len(packages) == 0 {
		return "", nil
	}
	return "RUN --mount=type=cache,target=/var/cache/apt apt-get update -qq && apt-get install -qqy " +
		strings.Join(packages, " ") +
		" && rm -rf /var/lib/apt/lists/*", nil
}

func (g *Generator) installPythonCUDA() (string, error) {
	// TODO: check that python version is valid

	py := g.Config.Build.PythonVersion

	return `ENV PATH="/root/.pyenv/shims:/root/.pyenv/bin:$PATH"
RUN --mount=type=cache,target=/var/cache/apt apt-get update -qq && apt-get install -qqy --no-install-recommends \
	make \
	build-essential \
	libssl-dev \
	zlib1g-dev \
	libbz2-dev \
	libreadline-dev \
	libsqlite3-dev \
	ifconfig \
	iproute2 \
	wget \
	curl \
	llvm \
	libncurses5-dev \
	libncursesw5-dev \
	xz-utils \
	tk-dev \
	libffi-dev \
	liblzma-dev \
	git \
	ca-certificates \
	&& rm -rf /var/lib/apt/lists/*
` + fmt.Sprintf(`RUN curl -s -S -L https://raw.githubusercontent.com/pyenv/pyenv-installer/master/bin/pyenv-installer | bash && \
	git clone https://github.com/momo-lab/pyenv-install-latest.git "$(pyenv root)"/plugins/pyenv-install-latest && \
	pyenv install-latest "%s" && \
	pyenv global $(pyenv install-latest --print "%s") && \
	pip install "wheel<1"`, py, py), nil
}

func (g *Generator) installCog() (string, error) {
	// Wheel name needs to be full format otherwise pip refuses to install it
	cogFilename := "cog-0.0.1.dev-py3-none-any.whl"
	cogPath := filepath.Join(g.tmpDir, cogFilename)
	if err := os.MkdirAll(filepath.Dir(cogPath), 0o755); err != nil {
		return "", fmt.Errorf("Failed to write %s: %w", cogFilename, err)
	}
	if err := os.WriteFile(cogPath, cogWheelEmbed, 0o644); err != nil {
		return "", fmt.Errorf("Failed to write %s: %w", cogFilename, err)
	}
	return fmt.Sprintf(`COPY %s /tmp/%s
RUN --mount=type=cache,target=/root/.cache/pip pip install /tmp/%s`, path.Join(g.relativeTmpDir, cogFilename), cogFilename, cogFilename), nil
}

func (g *Generator) installPydanticNoBinary() string {
	return "RUN --mount=type=cache,target=/root/.cache/pip pip install pydantic --no-binary :all:"
}

func (g *Generator) uninstallPydantic() string {
	return "RUN --mount=type=cache,target=/root/.cache/pip pip uninstall pydantic -y"
}

func (g *Generator) installCython() string {
	return "RUN --mount=type=cache,target=/root/.cache/pip pip install cython==\"0.29.34\""
}
func (g *Generator) installSieve() string {
	sieveExternal := "sievedata-0.0.1.1-py3-none-any.whl"
	format := "COPY %s /tmp/%s\n RUN --mount=type=cache,target=/root/.cache/pip pip install /tmp/%s"
	line2 := fmt.Sprintf(format, sieveExternal, sieveExternal, sieveExternal)
	return fmt.Sprintf("%s", line2)
}

func (g *Generator) pythonRequirements() (string, error) {
	reqs := g.Config.Build.PythonRequirements

	if reqs == "" {
		return "", nil
	}
	return fmt.Sprintf(`COPY %s /tmp/requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip pip install -r /tmp/requirements.txt && rm /tmp/requirements.txt`, reqs), nil
}

func (g *Generator) CogSHA256() string {
	return generateSHA256(cogWheelEmbed)
}

func generateSHA256(input []byte) string {
	// Create a new SHA256 hash object
	hash := sha256.New()

	// Write the input string to the hash object
	hash.Write([]byte(input))

	// Get the hash sum as a byte slice
	hashSum := hash.Sum(nil)

	// Convert the byte slice to a hexadecimal string
	hashString := hex.EncodeToString(hashSum)

	return hashString
}

func (g *Generator) sieveRequirements() string {
	return fmt.Sprintf(`RUN --mount=type=cache,target=/root/.cache/pip pip install %s`, strings.Join(SieveRequirements, " "))
}

func (g *Generator) pipInstalls() (string, error) {
	requirements, err := g.Config.PythonRequirementsForArch(g.GOOS, g.GOARCH)
	if err != nil {
		return "", err
	}
	if strings.Trim(requirements, "") == "" {
		return "", nil
	}

	lines, containerPath, err := g.writeTemp("requirements.txt", []byte(requirements))
	if err != nil {
		return "", err
	}

	lines = append(lines, "RUN --mount=type=cache,target=/root/.cache/pip pip install -r "+containerPath)
	return strings.Join(lines, "\n"), nil
}

func (g *Generator) run() (string, error) {
	runCommands := g.Config.Build.Run

	// For backwards compatibility
	runCommands = append(runCommands, g.Config.Build.PreInstall...)

	lines := []string{}
	for _, run := range runCommands {
		run = strings.TrimSpace(run)
		if strings.Contains(run, "\n") {
			return "", fmt.Errorf(`One of the commands in 'run' contains a new line, which won't work. You need to create a new list item in YAML prefixed with '-' for each command.

This is the offending line: %s`, run)
		}
		lines = append(lines, "RUN "+run)
	}
	return strings.Join(lines, "\n"), nil
}

// writeTemp writes a temporary file that can be used as part of the build process
// It returns the lines to add to Dockerfile to make it available and the filename it ends up as inside the container
func (g *Generator) writeTemp(filename string, contents []byte) ([]string, string, error) {
	path := filepath.Join(g.tmpDir, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return []string{}, "", fmt.Errorf("Failed to write %s: %w", filename, err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return []string{}, "", fmt.Errorf("Failed to write %s: %w", filename, err)
	}
	return []string{fmt.Sprintf("COPY %s /tmp/%s", filepath.Join(g.relativeTmpDir, filename), filename)}, "/tmp/" + filename, nil
}

func filterEmpty(list []string) []string {
	filtered := []string{}
	for _, s := range list {
		if s != "" {
			filtered = append(filtered, s)
		}
	}
	return filtered
}
