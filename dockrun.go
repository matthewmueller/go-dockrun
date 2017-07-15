package dockrun

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	docker "github.com/fsouza/go-dockerclient"
	multierror "github.com/hashicorp/go-multierror"
)

// Client struct
type Client struct {
	client *docker.Client
}

// Container struct
type Container struct {
	client *docker.Client
	image  string
	name   string
	expose []string
}

// Runner struct
type Runner struct {
	client    *docker.Client
	container *docker.Container
}

// New docker client
func New() (*Client, error) {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return nil, err
	}

	return &Client{
		client: client,
	}, nil
}

// Container creates a new container
func (c *Client) Container(image string, name string) *Container {
	return &Container{
		client: c.client,
		image:  image,
		name:   name,
	}
}

// Expose sets up a port mapping
func (c *Container) Expose(portmap string) *Container {
	c.expose = append(c.expose, portmap)
	return c
}

func (c *Container) ensure(image string) error {
	_, err := c.client.InspectImage(c.image)
	return err
}

// Run a command on the container
func (c *Container) Run(ctx context.Context, cmd ...string) (*Runner, error) {
	if e := c.ensure(c.image); e != nil {
		return nil, e
	}

	exposedPorts := make(map[docker.Port]struct{})
	portBindings := make(map[docker.Port][]docker.PortBinding)
	for _, port := range c.expose {
		bindings := strings.Split(port, ":")

		if len(bindings) == 1 {
			exposedPorts[docker.Port(port)] = struct{}{}
			continue
		}

		dockerPort := docker.Port(bindings[1])
		host := docker.PortBinding{
			HostIP:   "0.0.0.0",
			HostPort: bindings[0],
		}

		exposedPorts[dockerPort] = struct{}{}
		portBindings[dockerPort] = []docker.PortBinding{host}
	}

	container, err := c.client.CreateContainer(
		docker.CreateContainerOptions{
			Name: c.name,
			Config: &docker.Config{
				Image: c.image,
				Cmd:   cmd,
				// TODO: configurable
				// Env:          conf.Env,
				// Hostname:     conf.Hostname,
				// Domainname:   conf.Domainname,
				// User:         conf.User,
				Tty:          true,
				ExposedPorts: exposedPorts,
			},
			HostConfig: &docker.HostConfig{
				PortBindings: portBindings,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	err = c.client.StartContainerWithContext(container.ID, nil, ctx)
	if err != nil {
		return nil, err
	}

	cjson, err := c.client.InspectContainer(container.ID)
	if err != nil {
		return nil, err
	}

	return &Runner{
		client:    c.client,
		container: cjson,
	}, nil
}

// Stop the container and remove it
func (r *Runner) Stop(killDeadline uint) (err error) {
	if e := r.client.StopContainer(r.container.ID, killDeadline); e != nil {
		err = multierror.Append(err, e)
	}

	opts := docker.RemoveContainerOptions{
		ID:            r.container.ID,
		RemoveVolumes: true,
		Force:         true,
	}

	if e := r.client.RemoveContainer(opts); e != nil {
		err = multierror.Append(err, e)
	}

	return err
}

// Kill the container
func (r *Runner) Kill() (err error) {
	kopts := docker.KillContainerOptions{
		ID: r.container.ID,
	}

	if e := r.client.KillContainer(kopts); e != nil {
		err = multierror.Append(err, e)
	}

	ropts := docker.RemoveContainerOptions{
		ID:            r.container.ID,
		RemoveVolumes: true,
		Force:         true,
	}

	if e := r.client.RemoveContainer(ropts); e != nil {
		err = multierror.Append(err, e)
	}

	return err
}

// Wait for a container to become ready
func (r *Runner) Wait(ctx context.Context, addr string) (err error) {
	b := backo(ctx)

	u, err := url.Parse(addr)
	if err != nil {
		return err
	}

	for {
		switch u.Scheme {
		case "http", "https":
			resp, err := http.Get(u.String())
			if err == nil {
				return resp.Body.Close()
			}
		default:
			conn, err := net.Dial(u.Scheme, u.Host)
			if err == nil {
				return conn.Close()
			}
		}

		sleep := b.NextBackOff()
		if sleep == backoff.Stop {
			return ctx.Err()
		}
		time.Sleep(sleep)
	}
}

func backo(ctx context.Context) backoff.BackOffContext {
	return backoff.WithContext(backoff.NewExponentialBackOff(), ctx)
}
