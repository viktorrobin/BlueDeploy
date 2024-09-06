package deployment

import (
	"DeploymentManager/utils"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/go-connections/nat"
	"io"
	"log"
	"os"
)

// ImageSummary of a deployment image
type ImageSummary struct {
	Containers  int64             `json:"Containers"`
	Created     int64             `json:"Created"`
	ID          string            `json:"Id"`
	Labels      map[string]string `json:"Labels"`
	ParentID    string            `json:"ParentId"`
	RepoDigests []string          `json:"RepoDigests"`
	RepoTags    []string          `json:"RepoTags"`
	SharedSize  int64             `json:"SharedSize"`
	Size        int64             `json:"Size"`
	VirtualSize int64             `json:"VirtualSize"`
}

type dockerCmd struct {
	cli                *client.Client
	dockerHost         string
	registry           string
	registryAuthString string
	registryAuthConfig registry.AuthConfig
	registryAuthMap    map[string]registry.AuthConfig
	noCache            bool
	forceRm            bool
	pull               bool
}

// Configs are used to create the deployment client
type Configs struct {
	Host     string
	Registry string
	Username string
	Password string
}

// Docker is an interface that contains some operations which can be used to build an image from source code
type Docker interface {
	Build(ctx context.Context, contextDirectory, imagePath string, args map[string]*string) error
	Pull(ctx context.Context, imagePath string) error
	Push(ctx context.Context, imagePath string) error
	List(ctx context.Context, filters map[string]string) ([]*ImageSummary, error)
	Tag(ctx context.Context, imagePath, newImagePath string) error
	Rmi(ctx context.Context, imagePath string) error
	RegistryLogin(ctx context.Context) error
	DeployContainer(ctx context.Context, deploymentRequest DeploymentRequest) (string, error)
	RecreateRunningContainers(ctx context.Context) error
}

func (docker *dockerCmd) RegistryLogin(ctx context.Context) error {
	out, err := docker.cli.RegistryLogin(ctx, docker.registryAuthConfig)
	if err != nil {
		log.Panicln("Error logging into Docker registry: %v\n", err)
	} else {
		log.Println("Logged into Docker registry: ", out.Status)
	}

	return err
}

func (docker *dockerCmd) DeployContainer(ctx context.Context, req DeploymentRequest) (string, error) {

	// Cleanup: Remove exited containers
	go removeExitedContainers(ctx, docker.cli)

	// Pull image
	imageName := req.Container.Image
	containerName := req.Container.Name

	log.Println("Pulling image: ", imageName)
	err := docker.Pull(ctx, imageName)

	if err != nil {
		log.Printf("Error pulling from Docker registry: %v\n", err)
		return "", err
	}

	// Stop and remove containers using the same image
	log.Println("Stopping running container using image: ", imageName)
	err = docker.stopRunningContainersByImage(ctx, imageName, containerName)
	if err != nil {
		log.Printf("Error stopping containers: %v\n", err)
	}

	// Cleanup: Remove dangling images
	go removeDanglingImages(ctx, docker.cli)

	log.Println("Creating container...")
	// If there are environment variables, add them
	var envVars []string
	if req.Container.EnvVars != nil {
		for _, envVar := range req.Container.EnvVars {
			envVars = append(envVars, envVar.Name+"="+envVar.Value)
		}
	}

	// If secrets, add them. They are loaded from environment
	if req.Container.Secrets != nil {
		if req.Container.Secrets != nil {
			for _, secret := range req.Container.Secrets {
				envVars = append(envVars, secret.SecretKey+"="+os.Getenv(secret.SecretKey))
			}
		}
	}

	log.Println("Environment variables: ", envVars)

	// Initialise portBinding as nil
	containerPortBinding := nat.PortMap{}
	exposedPort := nat.PortSet{}

	if req.Container.ContainerPort != "" {
		exposedPort = map[nat.Port]struct{}{
			nat.Port(req.Container.ContainerPort + "/tcp"): {},
		}

		// Set port binding
		hostBinding := nat.PortBinding{
			HostIP:   req.Container.Binding,
			HostPort: req.Container.HostPort,
		}

		containerPortBinding = nat.PortMap{
			nat.Port(req.Container.ContainerPort + "/tcp"): []nat.PortBinding{hostBinding},
		}

	}
	// Create container config
	containerConfig := &containertypes.Config{
		Image:        imageName,
		Env:          envVars,
		ExposedPorts: exposedPort,
	}

	// Create host config
	hostConfig := &containertypes.HostConfig{
		PortBindings: containerPortBinding,
		RestartPolicy: containertypes.RestartPolicy{
			Name: containertypes.RestartPolicyAlways,
		},
	}

	// Create network config for existing network call "bluerobin"
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"bluerobin": {},
		},
	}

	// Check if network exists
	filtersNetwork := filters.NewArgs()
	filtersNetwork.Add("name", "bluerobin")

	listNetwork, _ := docker.cli.NetworkList(ctx, network.ListOptions{Filters: filtersNetwork})

	if len(listNetwork) == 0 {
		log.Println("Creating network: bluerobin")
		docker.cli.NetworkCreate(ctx, "bluerobin", network.CreateOptions{})
	}

	// Create new container
	resp, err := docker.cli.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		log.Printf("Error creating container: %v\n", err)
		return "", err
	}

	// Start the container
	if err := docker.cli.ContainerStart(ctx, resp.ID, containertypes.StartOptions{}); err != nil {
		log.Println("Error starting container: ", err)
		return "", err
	}

	return resp.ID, err
}

func (docker *dockerCmd) RecreateRunningContainers(ctx context.Context) error {
	// Stop and remove containers using the same image
	filterArgs := filters.NewArgs()
	filterArgs.Add("network", "bluerobin")
	containers, _ := docker.cli.ContainerList(ctx, containertypes.ListOptions{All: true, Filters: filterArgs})

	// Loop over each container, copy its configuration and recreate it
	for _, container := range containers {

		// Get the container configuration
		fileName := container.ID + ".gob"
		// Load the request object from directory /deployments
		var request DeploymentRequest
		err := utils.ReadFromFile(fileName, &request)
		if err != nil {
			fmt.Println("Error reading object:", err)
			return err
		}

		containerId, err := docker.DeployContainer(ctx, request)
		if err != nil {
			log.Printf("Error deploying container: %v\n", err)
			return err
		}

		// Save the request object to directory /deployments
		fileName = containerId + ".gob"
		err = utils.SaveToFile(fileName, request)
		if err != nil {
			fmt.Println("Error saving object:", err)
			return err
		}

		// Delete old file container.ID + ".gob"
		err = utils.DeleteFile(container.ID + ".gob")
		if err != nil {
			fmt.Println("Error deleting file:", err)
			return err
		}
	}

	return nil
}

func (docker *dockerCmd) stopRunningContainersByImage(ctx context.Context, imageName string, containerName string) error {
	// Check if a container with the same name already exists and stop it

	filtersArgsImage := filters.NewArgs()
	filtersArgsImage.Add("ancestor", imageName)

	filtersArgsName := filters.NewArgs()
	filtersArgsName.Add("name", containerName)

	listFilters := []filters.Args{filtersArgsImage, filtersArgsName}

	log.Printf("Checking for running containers using image '%v' or with name '%v'", imageName, containerName)

	for _, filter := range listFilters {

		containers, err := docker.cli.ContainerList(ctx, containertypes.ListOptions{All: true, Filters: filter})
		if err != nil {
			panic(err)
		}

		log.Printf("--> Found %v running containers", len(containers))

		for _, icontainer := range containers {

			if icontainer.Image == imageName || icontainer.Names[0] == "/"+containerName {

				noWaitTimeout := 0 // to not wait for the container to exit gracefully
				err := docker.cli.ContainerStop(
					ctx,
					icontainer.ID,
					containertypes.StopOptions{Timeout: &noWaitTimeout},
				)

				if err != nil {
					panic(err)
				}

				// Remove the container
				err = docker.cli.ContainerRemove(
					ctx,
					icontainer.ID,
					containertypes.RemoveOptions{Force: true},
				)

				if err != nil {
					log.Printf("Error removing container: %v\n", err)
				}
			}
		}
	}
	return nil
}

func (docker *dockerCmd) Build(ctx context.Context, contextDirectory, imagePath string, args map[string]*string) error {
	//TODO implement me
	panic("implement me")
}

// NewClient will return a deployment image builder client
func NewClient(cfg Configs) (Docker, error) {

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer cli.Close()
	if err != nil {
		return nil, err
	}

	auth := registry.AuthConfig{
		Username:      cfg.Username,
		Password:      cfg.Password,
		ServerAddress: cfg.Registry,
	}
	authBytes, _ := json.Marshal(auth)
	authBase64 := base64.URLEncoding.EncodeToString(authBytes)

	docker := &dockerCmd{
		cli:                cli,
		dockerHost:         cfg.Host,
		registry:           cfg.Registry,
		registryAuthString: authBase64,
		registryAuthConfig: auth,
		registryAuthMap: map[string]registry.AuthConfig{
			cfg.Registry: auth,
		},
		noCache: true,
		forceRm: true,
		pull:    true,
	}

	return docker, nil
}

func (docker *dockerCmd) Pull(ctx context.Context, imagePath string) error {

	out, err := docker.cli.ImagePull(ctx, imagePath, imagetypes.PullOptions{All: true, RegistryAuth: docker.registryAuthString})
	//io.Copy(os.Stdout, resp)

	if err != nil {
		return err
	}

	defer out.Close()

	// Read and discard the output to ensure the pull operation completes
	dec := json.NewDecoder(out)
	for {
		var v interface{}
		if err := dec.Decode(&v); err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Error decoding JSON stream: %v", err)
		}
	}

	log.Println("Image pull completed successfully.")

	return nil
}

func (docker *dockerCmd) Push(ctx context.Context, imagePath string) error {
	resp, err := docker.cli.ImagePush(ctx, imagePath, imagetypes.PushOptions{
		RegistryAuth: docker.registryAuthString,
	})

	if resp != nil {
		defer resp.Close()
	}

	if err != nil {
		return err
	}

	if err = detectErrorMessage(resp); err != nil {
		return err
	}

	return nil
}

func (docker *dockerCmd) List(ctx context.Context, filter map[string]string) ([]*ImageSummary, error) {
	args := filters.NewArgs()
	for k, v := range filter {
		args.Add(k, v)
	}
	imageSummaryList, err := docker.cli.ImageList(ctx, imagetypes.ListOptions{
		Filters: args,
	})
	if err != nil {
		return nil, err
	}
	var imageSummaryPointerList []*ImageSummary
	for _, summary := range imageSummaryList {
		imageSummaryPointerList = append(imageSummaryPointerList, &ImageSummary{
			Containers:  summary.Containers,
			Created:     summary.Created,
			ID:          summary.ID,
			Labels:      summary.Labels,
			ParentID:    summary.ParentID,
			RepoDigests: summary.RepoDigests,
			RepoTags:    summary.RepoTags,
			SharedSize:  summary.SharedSize,
			Size:        summary.Size,
			VirtualSize: summary.VirtualSize,
		})
	}
	return imageSummaryPointerList, nil
}
func (docker *dockerCmd) Tag(ctx context.Context, imagePath, newImagePath string) error {
	err := docker.cli.ImageTag(ctx, imagePath, newImagePath)
	if err != nil {
		return err
	}
	return nil
}
func (docker *dockerCmd) Rmi(ctx context.Context, imagePath string) error {
	_, err := docker.cli.ImageRemove(ctx, imagePath, imagetypes.RemoveOptions{})
	if err != nil {
		return err
	}
	return nil
}

func detectErrorMessage(in io.Reader) error {
	dec := json.NewDecoder(in)

	for {
		var jm jsonmessage.JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if jm.Error != nil {
			return jm.Error
		}

	}
	return nil
}

// Connect initializes the containers client
func Connect() (*client.Client, error) {
	var err error

	// Create a new Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer cli.Close()
	return cli, err
}

func AuthToken(authConfig registry.AuthConfig) string {

	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		panic(err)
	}

	return base64.URLEncoding.EncodeToString(encodedJSON)
}

func removeExitedContainers(ctx context.Context, cli *client.Client) {
	containers, err := cli.ContainerList(ctx, containertypes.ListOptions{All: true})
	if err != nil {
		log.Fatal(err)
	}

	for _, container := range containers {
		if container.State == "exited" {
			fmt.Printf("Removing container %s\n", container.ID)
			if err := cli.ContainerRemove(ctx, container.ID, containertypes.RemoveOptions{Force: true}); err != nil {
				log.Printf("Failed to remove container %s: %v\n", container.ID, err)
			}
		}
	}
}

func removeDanglingImages(ctx context.Context, cli *client.Client) {
	images, err := cli.ImageList(ctx, imagetypes.ListOptions{Filters: filters.NewArgs(filters.Arg("dangling", "true"))})
	if err != nil {
		log.Fatal(err)
	}

	for _, image := range images {
		fmt.Printf("Removing image %s\n", image.ID)
		if _, err := cli.ImageRemove(ctx, image.ID, imagetypes.RemoveOptions{Force: true}); err != nil {
			log.Printf("Failed to remove image %s: %v\n", image.ID, err)
		}
	}
}
