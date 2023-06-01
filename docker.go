package docker

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"time"
)

type Container struct {
	ImageToPull       string        // Docker image to be pulled
	HostPort          string        // Port to map with container, "9876" by default
	ContainerPort     string        // Port to map with host
	ContainerProtocol string        // "tcp" by default
	BindHostConfig    []string      // List of volume bindings for this container, e.g: []string {"/host/path/to/bind:/container/path/bind"}
	Env               []string      // Environments to be loaded into the container
	Cmd               []string      // Commands to be executed into the container after creation
	Sleep             time.Duration // Time given to container to be ready
	client            *client.Client
	id                string
}

func WithContainerProtocol(protocol string) func(*Container) {
	return func(c *Container) {
		c.ContainerProtocol = protocol
	}
}

func WithHostPort(hostPort string) func(*Container) {
	return func(c *Container) {
		c.HostPort = hostPort
	}
}

func WithContainerPort(containerPort string) func(*Container) {
	return func(c *Container) {
		c.ContainerPort = containerPort
	}
}

func WithBindHostConfig(bindHostConfig []string) func(*Container) {
	return func(c *Container) {
		c.BindHostConfig = bindHostConfig
	}
}

func WithEnv(env []string) func(*Container) {
	return func(c *Container) {
		c.Env = env
	}
}

func WithCmd(cmd []string) func(*Container) {
	return func(c *Container) {
		c.Cmd = cmd
	}
}

func WithSleep(sleepTime time.Duration) func(c *Container) {
	return func(c *Container) {
		c.Sleep = sleepTime
	}
}

func NewContainer(imageToPull, containerPort string, options ...func(config *Container)) (*Container, error) {
	if imageToPull == "" {
		return nil, errors.New("imageToPull cannot be empty")
	}
	if containerPort == "" {
		return nil, errors.New("containerPort cannot be empty")
	}

	conf := &Container{
		ImageToPull:       imageToPull,
		HostPort:          "9876",
		ContainerPort:     containerPort,
		ContainerProtocol: "tcp",
	}
	for _, opt := range options {
		opt(conf)
	}
	return conf, nil
}

func (c *Container) CreateContainer() error {
	//new docker API client
	cli, err := client.NewClientWithOpts()
	if err != nil {
		return errors.Wrap(err, "unable to create docker client")
	}
	//Mapping ports
	hostBinding := nat.PortBinding{
		HostIP:   "127.0.0.1",
		HostPort: c.HostPort,
	}
	containerPort, err := nat.NewPort(c.ContainerProtocol, c.ContainerPort)
	if err != nil {
		return errors.Wrap(err, "unable to get port")
	}
	portBinding := nat.PortMap{containerPort: []nat.PortBinding{hostBinding}}

	ctx := context.Background()
	//Pulling imageToPull...
	_, err = cli.ImagePull(ctx, c.ImageToPull, types.ImagePullOptions{})
	if err != nil {
		return errors.Wrap(err, "unable to pull image")
	}

	cont, err := cli.ContainerCreate(
		context.Background(),
		&container.Config{
			AttachStdout: true,
			AttachStderr: true,
			Env:          c.Env,
			Image:        c.ImageToPull,
		},
		&container.HostConfig{
			PortBindings: portBinding,
			Binds:        c.BindHostConfig,
		}, nil, nil, "")

	if err != nil {
		return errors.Wrap(err, "unable to create container")
	}

	err = cli.ContainerStart(ctx, cont.ID, types.ContainerStartOptions{})
	if err != nil {
		return errors.Wrap(err, "unable to start container")
	}

	time.Sleep(c.Sleep)

	if err = executeCommands(ctx, cli, cont.ID, c.Cmd); err != nil {
		return errors.Wrap(err, "commands were not executed")
	}

	c.id = cont.ID
	c.client = cli

	return nil
}

func executeCommands(ctx context.Context, cli *client.Client, id string, cmd []string) error {
	if cmd == nil {
		return nil
	}

	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}
	execID, err := cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return errors.Wrap(err, "unable to create exec configuration")
	}

	//Attaching connection to get exec logs
	response, err := cli.ContainerExecAttach(context.Background(), execID.ID, types.ExecStartCheck{})
	if err != nil {
		return errors.Wrap(err, "unable to attach connection")
	}
	defer response.Close()

	if err = cli.ContainerExecStart(ctx, execID.ID, types.ExecStartCheck{}); err != nil {
		return errors.Wrap(err, "unable to start exec")
	}

	data, _ := ioutil.ReadAll(response.Reader)
	log.Println(string(data))
	return nil
}

func (c *Container) Stop() {
	if err := c.client.ContainerStop(context.Background(), c.id, container.StopOptions{}); err != nil {
		log.Printf("unable to stop container: %v", err)
	}
	err := c.client.ContainerRemove(context.Background(), c.id, types.ContainerRemoveOptions{})
	if err != nil {
		log.Printf("unable to remove container: %v", err)
	}
}
