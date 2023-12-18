package clients

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/securesign/sigstore-e2e/pkg/support"

	"github.com/securesign/sigstore-e2e/pkg/kubernetes"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

type cli struct {
	Name      string
	pathToCLI string
	setup     SetupStrategy
}

type SetupStrategy func(context.Context, *cli) (string, error)

func (c *cli) Command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.pathToCLI, args...) // #nosec G204 - we don't expect the code to be running on PROD ENV

	cmd.Stdout = logrus.NewEntry(logrus.StandardLogger()).WithField("app", c.Name).WriterLevel(logrus.InfoLevel)
	cmd.Stderr = logrus.NewEntry(logrus.StandardLogger()).WithField("app", c.Name).WriterLevel(logrus.ErrorLevel)

	return cmd
}

func (c *cli) CommandOutput(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.pathToCLI, args...) // #nosec G204 - we don't expect the code to be running on PROD ENV
	return cmd.Output()
}

func (c *cli) WithSetupStrategy(strategy SetupStrategy) *cli {
	c.setup = strategy
	return c
}

func (c *cli) Setup(ctx context.Context) error {
	var err error
	c.pathToCLI, err = c.setup(ctx, c)
	return err
}

func (c *cli) Destroy(_ context.Context) error {
	return nil
}

func BuildFromGit(url string, branch string) SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		dir, _, err := support.GitClone(url, branch)
		if err != nil {
			return "", err
		}
		err = exec.Command("go", "build", "-C", dir, "-o", c.Name, "./cmd/"+c.Name).Run()
		return dir + "/" + c.Name, err
	}

}

func DownloadFromOpenshift(consoleCliDownloadName string) SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		// Get http link
		link, err := kubernetes.ConsoleCLIDownload(ctx, consoleCliDownloadName, runtime.GOOS)
		if err != nil {
			return "", err
		}

		tmp, err := os.MkdirTemp("", consoleCliDownloadName)
		if err != nil {
			return "", err
		}

		logrus.Info("Downloading ", consoleCliDownloadName, " from ", link)
		fileName := tmp + string(os.PathSeparator) + consoleCliDownloadName
		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0711)
		if err != nil {
			return "", err
		}
		defer file.Close()

		if err = support.DownloadAndUnzip(ctx, link, file); err != nil {
			return "", err
		}

		return file.Name(), err
	}

}

func LocalBinary() SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		return exec.LookPath(c.Name)
	}

}

func ExtractFromContainer(image string, path string) SetupStrategy {
	return func(ctx context.Context, c *cli) (string, error) {
		dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return "", err
		}

		registryAuth, err := support.DockerAuth()
		if err != nil {
			return "", err
		}
		pull, err := dockerCli.ImagePull(ctx, image, types.ImagePullOptions{RegistryAuth: registryAuth})
		if err != nil {
			return "", err
		}
		defer pull.Close()
		out := logrus.NewEntry(logrus.StandardLogger()).WithField("app", "docker").WriterLevel(logrus.DebugLevel)
		_, _ = io.Copy(out, pull)

		var cont container.ContainerCreateCreatedBody
		if cont, err = dockerCli.ContainerCreate(ctx, &container.Config{Image: image},
			nil,
			nil,
			&v1.Platform{OS: runtime.GOOS},
			uuid.New().String()); err != nil {
			return "", err
		}

		var tarOut io.ReadCloser
		if tarOut, _, err = dockerCli.CopyFromContainer(ctx, cont.ID, path); err != nil {
			return "", err
		}

		defer tarOut.Close()

		cliName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		tmp, err := os.MkdirTemp("", cliName)
		if err != nil {
			return "", err
		}
		fileName := tmp + string(os.PathSeparator) + cliName
		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0711)
		if err != nil {
			return "", err
		}
		defer file.Close()

		r, w := io.Pipe()
		defer r.Close()

		go func() {
			defer w.Close()
			if err := support.UntarFile(tarOut, w); err != nil {
				panic(err)

			}
		}()

		if err = support.Gunzip(r, file); err != nil {
			return "", err
		}
		return file.Name(), err
	}
}
