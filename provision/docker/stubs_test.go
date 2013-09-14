// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"fmt"
	"github.com/dotcloud/docker"
	"github.com/globocom/config"
	"github.com/globocom/docker-cluster/cluster"
	etesting "github.com/globocom/tsuru/exec/testing"
	rtesting "github.com/globocom/tsuru/router/testing"
	"labix.org/v2/mgo/bson"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

var inspectOut = `
{
	"State": {
		"Running": false,
		"Pid": 0,
		"ExitCode": 0,
		"StartedAt": "2013-06-13T20:59:31.699407Z",
		"Ghost": false
	},
	"NetworkSettings": {
		"IpAddress": "10.10.10.10",
		"IpPrefixLen": 8,
		"Gateway": "10.65.41.1",
		"PortMapping": {"8888": "34233"}
	}
}`

func createTestRoutes(names ...string) func() {
	for _, name := range names {
		rtesting.FakeRouter.AddBackend(name)
	}
	return func() {
		for _, name := range names {
			rtesting.FakeRouter.RemoveBackend(name)
		}
	}
}

func startTestListener(addr string) net.Listener {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	return listener
}

func startDockerTestServer(containerPort string, calls *int) (func(), *httptest.Server) {
	listAllOutput := `[
    {
        "Id": "8dfafdbc3a40",
        "Image": "base:latest",
        "Command": "echo 1",
        "Created": 1367854155,
        "Status": "Ghost",
        "Ports": null,
        "SizeRw":12288,
        "SizeRootFs": 0
    },
    {
        "Id": "dca19cd9bb9e",
        "Image": "tsuru/python:latest",
        "Command": "echo 1",
        "Created": 1376319760,
        "Status": "Exit 0",
        "Ports": null,
        "SizeRw": 0,
        "SizeRootFs": 0
    },
    {
        "Id": "3fd99cd9bb84",
        "Image": "tsuru/python:latest",
        "Command": "echo 1",
        "Created": 1376319760,
        "Status": "Up 7 seconds",
        "Ports": null,
        "SizeRw": 0,
        "SizeRootFs": 0
    }
]`
	c1Output := fmt.Sprintf(`{
    "State": {
        "Running": true,
        "Pid": 2785,
        "ExitCode": 0,
        "StartedAt": "2013-08-15T03:38:45.709874216-03:00",
        "Ghost": false
    },
	"NetworkSettings": {
		"IpAddress": "127.0.0.4",
		"IpPrefixLen": 8,
		"Gateway": "10.65.41.1",
		"PortMapping": {
			"Tcp": {"8888": "%s"}
		}
	}
}`, containerPort)
	c2Output := `{
    "State": {
        "Running": true,
        "Pid": 2785,
        "ExitCode": 0,
        "StartedAt": "2013-08-15T03:38:45.709874216-03:00",
        "Ghost": false
    },
    "Image": "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
	"NetworkSettings": {
		"IpAddress": "127.0.0.1",
		"IpPrefixLen": 8,
		"Gateway": "10.65.41.1",
		"PortMapping": {
			"Tcp": {"8889": "9024"}
		}
	}
}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		(*calls)++
		if strings.Contains(r.URL.Path, "/containers/") {
			if strings.Contains(r.URL.Path, "/containers/9930c24f1c4f") {
				w.Write([]byte(c2Output))
			}
			if strings.Contains(r.URL.Path, "/containers/9930c24f1c5f") {
				w.Write([]byte(c1Output))
			}
			if strings.Contains(r.URL.Path, "/containers/json") {
				w.Write([]byte(listAllOutput))
			}
			if strings.Contains(r.URL.Path, "/export") {
				w.Write([]byte("tar stream data"))
			}
		}
		if strings.Contains(r.URL.Path, "/commit") {
			w.Write([]byte(`{"Id":"i-1"}`))
		}
	}))
	var err error
	oldCluster := dockerCluster()
	dCluster, err = cluster.New(nil,
		cluster.Node{ID: "server", Address: server.URL},
	)
	if err != nil {
		panic(err)
	}
	return func() {
		server.Close()
		dCluster = oldCluster
	}, server
}

func startSSHAgentServer(output string) (*FakeSSHServer, func()) {
	var handler FakeSSHServer
	handler.output = output
	server := httptest.NewServer(&handler)
	_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	portNumber, _ := strconv.Atoi(port)
	config.Set("docker:ssh-agent-port", portNumber)
	return &handler, func() {
		server.Close()
		config.Unset("docker:ssh-agent-port")
	}
}

func insertContainers(containerPort string) func() {
	err := collection().Insert(
		container{
			ID: "9930c24f1c5f", AppName: "ashamed", Type: "python",
			Port: "8888", Status: "running", IP: "127.0.0.3",
			HostPort: "9023", HostAddr: "127.0.0.1",
		},
		container{
			ID: "9930c24f1c4f", AppName: "make-up", Type: "python",
			Port: "8889", Status: "running", IP: "127.0.0.4",
			HostPort: "9025", HostAddr: "127.0.0.1",
		},
		container{ID: "9930c24f1c6f", AppName: "make-up", Type: "python", Port: "9090", Status: "error", HostAddr: "127.0.0.1"},
		container{ID: "9930c24f1c7f", AppName: "make-up", Type: "python", Port: "9090", Status: "created", HostAddr: "127.0.0.1"},
	)
	if err != nil {
		panic(err)
	}
	rtesting.FakeRouter.AddRoute("ashamed", fmt.Sprintf("http://127.0.0.1:%s", containerPort))
	rtesting.FakeRouter.AddRoute("make-up", "http://127.0.0.1:9025")
	return func() {
		collection().RemoveAll(bson.M{"appname": "make-up"})
		collection().RemoveAll(bson.M{"appname": "ashamed"})
	}
}

func mockExecutor() (*etesting.FakeExecutor, func()) {
	fexec := &etesting.FakeExecutor{Output: map[string][][]byte{}}
	setExecut(fexec)
	return fexec, func() {
		setExecut(nil)
	}
}

type mapStorage struct {
	containers map[string]string
}

func (m *mapStorage) StoreContainer(containerID, hostID string) error {
	if m.containers == nil {
		m.containers = make(map[string]string)
	}
	m.containers[containerID] = hostID
	return nil
}

func (m *mapStorage) RetrieveContainer(containerID string) (string, error) {
	return m.containers[containerID], nil
}

func (m *mapStorage) RemoveContainer(containerID string) error {
	delete(m.containers, containerID)
	return nil
}

func (m *mapStorage) StoreImage(imageID, hostID string) error      { return nil }
func (m *mapStorage) RetrieveImage(imageID string) (string, error) { return "", nil }
func (m *mapStorage) RemoveImage(imageID string) error             { return nil }

type fakeScheduler struct {
	nodes     []cluster.Node
	container *docker.Container
}

func (s *fakeScheduler) Nodes() ([]cluster.Node, error) {
	return s.nodes, nil
}

func (s *fakeScheduler) Schedule(config *docker.Config) (string, *docker.Container, error) {
	return "server", s.container, nil
}

func (s *fakeScheduler) Register(nodes ...cluster.Node) error {
	s.nodes = append(s.nodes, nodes...)
	return nil
}
