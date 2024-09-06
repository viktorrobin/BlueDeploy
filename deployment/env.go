package deployment

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
)

// GetContainerEnv retrieves the list of environment variables of a running container
func GetContainerEnv(ctx context.Context, cli *client.Client, containerID string) ([]string, error) {
	containerJSON, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("error inspecting container: %w", err)
	}

	return containerJSON.Config.Env, nil
}
