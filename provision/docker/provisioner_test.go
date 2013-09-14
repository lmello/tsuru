// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	dockerClient "github.com/fsouza/go-dockerclient"
	"github.com/globocom/config"
	"github.com/globocom/tsuru/app"
	"github.com/globocom/tsuru/cmd"
	"github.com/globocom/tsuru/exec"
	etesting "github.com/globocom/tsuru/exec/testing"
	"github.com/globocom/tsuru/log"
	"github.com/globocom/tsuru/provision"
	"github.com/globocom/tsuru/queue"
	rtesting "github.com/globocom/tsuru/router/testing"
	"github.com/globocom/tsuru/testing"
	"labix.org/v2/mgo/bson"
	"launchpad.net/gocheck"
	stdlog "log"
	"net"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func setExecut(e exec.Executor) {
	emutex.Lock()
	execut = e
	emutex.Unlock()
}

func (s *S) TestShouldBeRegistered(c *gocheck.C) {
	p, err := provision.Get("docker")
	c.Assert(err, gocheck.IsNil)
	c.Assert(p, gocheck.FitsTypeOf, &dockerProvisioner{})
}

func (s *S) TestProvisionerProvision(c *gocheck.C) {
	app := testing.NewFakeApp("myapp", "python", 1)
	var p dockerProvisioner
	err := p.Provision(app)
	c.Assert(err, gocheck.IsNil)
	c.Assert(rtesting.FakeRouter.HasBackend("myapp"), gocheck.Equals, true)
	c.Assert(app.IsReady(), gocheck.Equals, true)
}

func (s *S) TestProvisionerRestartCallsTheRestartHook(c *gocheck.C) {
	var handler FakeSSHServer
	handler.output = "caad7bbd5411"
	server := httptest.NewServer(&handler)
	defer server.Close()
	host, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	portNumber, _ := strconv.Atoi(port)
	config.Set("docker:ssh-agent-port", portNumber)
	defer config.Unset("docker:ssh-agent-port")
	var p dockerProvisioner
	app := testing.NewFakeApp("almah", "static", 1)
	cont := container{
		ID:       handler.output,
		AppName:  app.GetName(),
		Type:     app.GetPlatform(),
		IP:       "10.10.10.10",
		HostAddr: host,
	}
	err := collection().Insert(cont)
	c.Assert(err, gocheck.IsNil)
	defer collection().RemoveId(cont.ID)
	err = p.Restart(app)
	c.Assert(err, gocheck.IsNil)
	input := cmdInput{Cmd: "/var/lib/tsuru/restart"}
	body := handler.bodies[0]
	c.Assert(body, gocheck.DeepEquals, input)
	path := "/container/10.10.10.10/cmd"
	c.Assert(handler.requests[0].URL.Path, gocheck.DeepEquals, path)
}

func (s *S) stopContainers(n uint) {
	client, err := dockerClient.NewClient(s.server.URL())
	if err != nil {
		return
	}
	for n > 0 {
		opts := dockerClient.ListContainersOptions{All: false}
		containers, err := client.ListContainers(opts)
		if err != nil {
			return
		}
		if len(containers) > 0 {
			for _, cont := range containers {
				if cont.ID != "" {
					client.StopContainer(cont.ID, 1)
					n--
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *S) TestDeploy(c *gocheck.C) {
	go s.stopContainers(1)
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, gocheck.IsNil)
	fexec := &etesting.FakeExecutor{}
	setExecut(fexec)
	defer setExecut(nil)
	p := dockerProvisioner{}
	app := testing.NewFakeApp("cribcaged", "python", 1)
	p.Provision(app)
	defer p.Destroy(app)
	w := writer{b: make([]byte, 2048)}
	err = p.Deploy(app, "master", &w)
	c.Assert(err, gocheck.IsNil)
	w.b = nil
	defer p.Destroy(app)
	time.Sleep(6e9)
	q, err := getQueue()
	message, err := q.Get(1e6)
	c.Assert(err, gocheck.IsNil)
	defer message.Delete()
	c.Assert(app.GetCommands(), gocheck.DeepEquals, []string{"serialize", "restart"})
	c.Assert(app.HasLog("tsuru", "Restarting app..."), gocheck.Equals, true)
}

func getQueue() (queue.Q, error) {
	queueName := "tsuru-app"
	qfactory, err := queue.Factory()
	if err != nil {
		return nil, err
	}
	return qfactory.Get(queueName)
}

func (s *S) TestDeployEnqueuesBindService(c *gocheck.C) {
	go s.stopContainers(1)
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, gocheck.IsNil)
	setExecut(&etesting.FakeExecutor{})
	defer setExecut(nil)
	p := dockerProvisioner{}
	a := testing.NewFakeApp("cribcaged", "python", 1)
	p.Provision(a)
	defer p.Destroy(a)
	w := writer{b: make([]byte, 2048)}
	err = p.Deploy(a, "master", &w)
	c.Assert(err, gocheck.IsNil)
	defer p.Destroy(a)
	q, err := getQueue()
	message, err := q.Get(1e6)
	c.Assert(err, gocheck.IsNil)
	defer message.Delete()
	c.Assert(err, gocheck.IsNil)
	c.Assert(message.Action, gocheck.Equals, app.BindService)
	c.Assert(message.Args[0], gocheck.Equals, a.GetName())
	c.Assert(message.Args[1], gocheck.Not(gocheck.Equals), "")
}

type writer struct {
	b   []byte
	cur int
}

func (w *writer) Write(c []byte) (int, error) {
	copy(w.b[w.cur:], c)
	w.cur += len(c)
	return len(c), nil
}

func (s *S) TestDeployRemoveContainersEvenWhenTheyreNotInTheAppsCollection(c *gocheck.C) {
	go s.stopContainers(3)
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, gocheck.IsNil)
	cont1, err := s.newContainer()
	c.Assert(err, gocheck.IsNil)
	_, err = s.newContainer()
	c.Assert(err, gocheck.IsNil)
	defer rtesting.FakeRouter.RemoveBackend(cont1.AppName)
	var p dockerProvisioner
	app := testing.NewFakeApp(cont1.AppName, "python", 0)
	p.Provision(app)
	defer p.Destroy(app)
	fexec := &etesting.FakeExecutor{}
	setExecut(fexec)
	defer setExecut(nil)
	var w bytes.Buffer
	err = p.Deploy(app, "master", &w)
	c.Assert(err, gocheck.IsNil)
	time.Sleep(1e9)
	defer p.Destroy(app)
	q, err := getQueue()
	message, err := q.Get(1e6)
	c.Assert(err, gocheck.IsNil)
	defer message.Delete()
	message, err = q.Get(1e6)
	c.Assert(err, gocheck.IsNil)
	defer message.Delete()
	n, err := s.conn.Collection(s.collName).Find(bson.M{"appname": cont1.AppName}).Count()
	c.Assert(err, gocheck.IsNil)
	c.Assert(n, gocheck.Equals, 2)
}

func (s *S) TestProvisionerDestroy(c *gocheck.C) {
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, gocheck.IsNil)
	cont, err := s.newContainer()
	c.Assert(err, gocheck.IsNil)
	app := testing.NewFakeApp(cont.AppName, "python", 1)
	var p dockerProvisioner
	p.Provision(app)
	c.Assert(p.Destroy(app), gocheck.IsNil)
	ok := make(chan bool, 1)
	go func() {
		coll := s.conn.Collection(s.collName)
		for {
			ct, err := coll.Find(bson.M{"appname": cont.AppName}).Count()
			if err != nil {
				c.Fatal(err)
			}
			if ct == 0 {
				ok <- true
				return
			}
			runtime.Gosched()
		}
	}()
	select {
	case <-ok:
	case <-time.After(10e9):
		c.Fatal("Timed out waiting for the container to be destroyed (10 seconds)")
	}
	c.Assert(rtesting.FakeRouter.HasBackend("myapp"), gocheck.Equals, false)
}

func (s *S) TestProvisionerDestroyEmptyUnit(c *gocheck.C) {
	fexec := &etesting.FakeExecutor{}
	setExecut(fexec)
	defer setExecut(nil)
	w := new(bytes.Buffer)
	l := stdlog.New(w, "", stdlog.LstdFlags)
	log.SetLogger(l)
	app := testing.NewFakeApp("myapp", "python", 0)
	app.AddUnit(&testing.FakeUnit{})
	var p dockerProvisioner
	p.Provision(app)
	err := p.Destroy(app)
	c.Assert(err, gocheck.IsNil)
}

func (s *S) TestProvisionerDestroyRemovesRouterBackend(c *gocheck.C) {
	fexec := &etesting.FakeExecutor{}
	setExecut(fexec)
	defer setExecut(nil)
	app := testing.NewFakeApp("myapp", "python", 0)
	var p dockerProvisioner
	err := p.Provision(app)
	c.Assert(err, gocheck.IsNil)
	err = p.Destroy(app)
	c.Assert(err, gocheck.IsNil)
	c.Assert(rtesting.FakeRouter.HasBackend("myapp"), gocheck.Equals, false)
}

func (s *S) TestProvisionerAddr(c *gocheck.C) {
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, gocheck.IsNil)
	cont, err := s.newContainer()
	c.Assert(err, gocheck.IsNil)
	defer cont.remove()
	app := testing.NewFakeApp(cont.AppName, "python", 1)
	defer rtesting.FakeRouter.RemoveBackend(app.GetName())
	var p dockerProvisioner
	addr, err := p.Addr(app)
	c.Assert(err, gocheck.IsNil)
	r, err := getRouter()
	c.Assert(err, gocheck.IsNil)
	expected, err := r.Addr(cont.AppName)
	c.Assert(err, gocheck.IsNil)
	c.Assert(addr, gocheck.Equals, expected)
}

func (s *S) TestProvisionerAddUnits(c *gocheck.C) {
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, gocheck.IsNil)
	var p dockerProvisioner
	app := testing.NewFakeApp("myapp", "python", 0)
	p.Provision(app)
	defer p.Destroy(app)
	s.conn.Collection(s.collName).Insert(container{ID: "c-89320", AppName: app.GetName(), Version: "a345fe"})
	defer s.conn.Collection(s.collName).RemoveId("c-89320")
	units, err := p.AddUnits(app, 3)
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Collection(s.collName).RemoveAll(bson.M{"appname": app.GetName()})
	c.Assert(units, gocheck.HasLen, 3)
	count, err := s.conn.Collection(s.collName).Find(bson.M{"appname": app.GetName()}).Count()
	c.Assert(err, gocheck.IsNil)
	c.Assert(count, gocheck.Equals, 4)
}

func (s *S) TestProvisionerAddZeroUnits(c *gocheck.C) {
	var p dockerProvisioner
	units, err := p.AddUnits(nil, 0)
	c.Assert(units, gocheck.IsNil)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "Cannot add 0 units")
}

func (s *S) TestProvisionerAddUnitsWithoutContainers(c *gocheck.C) {
	app := testing.NewFakeApp("myapp", "python", 1)
	var p dockerProvisioner
	p.Provision(app)
	defer p.Destroy(app)
	units, err := p.AddUnits(app, 1)
	c.Assert(units, gocheck.IsNil)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "New units can only be added after the first deployment")
}

func (s *S) TestProvisionerRemoveUnit(c *gocheck.C) {
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, gocheck.IsNil)
	container, err := s.newContainer()
	c.Assert(err, gocheck.IsNil)
	defer rtesting.FakeRouter.RemoveBackend(container.AppName)
	client, err := dockerClient.NewClient(s.server.URL())
	c.Assert(err, gocheck.IsNil)
	err = client.StartContainer(container.ID)
	c.Assert(err, gocheck.IsNil)
	app := testing.NewFakeApp(container.AppName, "python", 0)
	var p dockerProvisioner
	err = p.RemoveUnit(app, container.ID)
	c.Assert(err, gocheck.IsNil)
	_, err = getContainer(container.ID)
	c.Assert(err, gocheck.NotNil)
	images, err := client.ListImages(true)
	c.Assert(err, gocheck.IsNil)
	for _, image := range images {
		c.Assert(image.Repository, gocheck.Not(gocheck.Equals), "tsuru/python")
	}
}

func (s *S) TestProvisionerRemoveUnitNotFound(c *gocheck.C) {
	var p dockerProvisioner
	err := p.RemoveUnit(nil, "not-found")
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "not found")
}

func (s *S) TestProvisionerRemoveUnitNotInApp(c *gocheck.C) {
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, gocheck.IsNil)
	container, err := s.newContainer()
	c.Assert(err, gocheck.IsNil)
	defer rtesting.FakeRouter.RemoveBackend(container.AppName)
	defer container.remove()
	var p dockerProvisioner
	err = p.RemoveUnit(testing.NewFakeApp("hisapp", "python", 1), container.ID)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "Unit does not belong to this app")
	_, err = getContainer(container.ID)
	c.Assert(err, gocheck.IsNil)
}

func (s *S) TestRemoveUnitInSameHostAsAnotherUnitShouldEnqueueAnotherBind(c *gocheck.C) {
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, gocheck.IsNil)
	c1, err := s.newContainer()
	c.Assert(err, gocheck.IsNil)
	c2, err := s.newContainer()
	c.Assert(err, gocheck.IsNil)
	defer rtesting.FakeRouter.RemoveBackend(c1.AppName)
	client, err := dockerClient.NewClient(s.server.URL())
	c.Assert(err, gocheck.IsNil)
	err = client.StartContainer(c1.ID)
	c.Assert(err, gocheck.IsNil)
	a := testing.NewFakeApp(c1.AppName, "python", 0)
	var p dockerProvisioner
	err = p.RemoveUnit(a, c1.ID)
	c.Assert(err, gocheck.IsNil)
	_, err = getContainer(c1.ID)
	c.Assert(err, gocheck.NotNil)
	_, err = getContainer(c2.ID)
	c.Assert(err, gocheck.IsNil)
	q, err := getQueue()
	c.Assert(err, gocheck.IsNil)
	message, err := q.Get(1e6)
	c.Assert(err, gocheck.IsNil)
	expected := &queue.Message{Action: app.BindService, Args: []string{a.GetName(), c2.ID}}
	c.Assert(message, gocheck.DeepEquals, expected)
}

func (s *S) TestProvisionerExecuteCommand(c *gocheck.C) {
	var handler FakeSSHServer
	handler.output = ". .."
	server := httptest.NewServer(&handler)
	defer server.Close()
	host, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	portNumber, _ := strconv.Atoi(port)
	config.Set("docker:ssh-agent-port", portNumber)
	defer config.Unset("docker:ssh-agent-port")
	app := testing.NewFakeApp("starbreaker", "python", 1)
	container := container{ID: "c-036", AppName: "starbreaker", Type: "python", IP: "10.10.10.1", HostAddr: host}
	err := s.conn.Collection(s.collName).Insert(container)
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Collection(s.collName).Remove(bson.M{"_id": container.ID})
	var stdout, stderr bytes.Buffer
	var p dockerProvisioner
	err = p.ExecuteCommand(&stdout, &stderr, app, "ls", "-ar")
	c.Assert(err, gocheck.IsNil)
	c.Assert(stderr.Bytes(), gocheck.IsNil)
	c.Assert(stdout.String(), gocheck.Equals, ". ..")
	body := handler.bodies[0]
	input := cmdInput{Cmd: "ls", Args: []string{"-ar"}}
	c.Assert(body, gocheck.DeepEquals, input)
	path := "/container/10.10.10.1/cmd"
	c.Assert(handler.requests[0].URL.Path, gocheck.DeepEquals, path)
}

func (s *S) TestProvisionerExecuteCommandMultipleContainers(c *gocheck.C) {
	var handler FakeSSHServer
	handler.output = ". .."
	server := httptest.NewServer(&handler)
	defer server.Close()
	host, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	portNumber, _ := strconv.Atoi(port)
	config.Set("docker:ssh-agent-port", portNumber)
	defer config.Unset("docker:ssh-agent-port")
	app := testing.NewFakeApp("starbreaker", "python", 1)
	err := s.conn.Collection(s.collName).Insert(
		container{ID: "c-036", AppName: "starbreaker", Type: "python", IP: "10.10.10.1", HostAddr: host},
		container{ID: "c-037", AppName: "starbreaker", Type: "python", IP: "10.10.10.2", HostAddr: host},
	)
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Collection(s.collName).RemoveAll(bson.M{"_id": bson.M{"$in": []string{"c-036", "c-037"}}})
	var stdout, stderr bytes.Buffer
	var p dockerProvisioner
	err = p.ExecuteCommand(&stdout, &stderr, app, "ls", "-ar")
	c.Assert(err, gocheck.IsNil)
	c.Assert(stderr.Bytes(), gocheck.IsNil)
	input := cmdInput{Cmd: "ls", Args: []string{"-ar"}}
	c.Assert(handler.bodies, gocheck.DeepEquals, []cmdInput{input, input})
	path1 := "/container/10.10.10.1/cmd"
	path2 := "/container/10.10.10.2/cmd"
	c.Assert(handler.requests[0].URL.Path, gocheck.Equals, path1)
	c.Assert(handler.requests[1].URL.Path, gocheck.Equals, path2)
}

func (s *S) TestProvisionerExecuteCommandNoContainers(c *gocheck.C) {
	var p dockerProvisioner
	app := testing.NewFakeApp("almah", "static", 2)
	var buf bytes.Buffer
	err := p.ExecuteCommand(&buf, &buf, app, "ls", "-lh")
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "No containers for this app")
}

func (s *S) TestCollectStatus(c *gocheck.C) {
	defer createTestRoutes("ashamed", "make-up")()
	listener := startTestListener("127.0.0.1:0")
	defer listener.Close()
	listenPort := strings.Split(listener.Addr().String(), ":")[1]
	var calls int
	var err error
	cleanup, _ := startDockerTestServer(listenPort, &calls)
	defer cleanup()
	sshHandler, cleanup := startSSHAgentServer("")
	defer cleanup()
	defer insertContainers(listenPort)()
	expected := []provision.Unit{
		{
			Name:    "9930c24f1c5f",
			AppName: "ashamed",
			Type:    "python",
			Machine: 0,
			Ip:      "127.0.0.1",
			Status:  provision.StatusStarted,
		},
		{
			Name:    "9930c24f1c4f",
			AppName: "make-up",
			Type:    "python",
			Machine: 0,
			Ip:      "127.0.0.1",
			Status:  provision.StatusInstalling,
		},
		{
			Name:    "9930c24f1c6f",
			AppName: "make-up",
			Type:    "python",
			Status:  provision.StatusError,
		},
	}
	var p dockerProvisioner
	units, err := p.CollectStatus()
	c.Assert(err, gocheck.IsNil)
	sortUnits(units)
	sortUnits(expected)
	c.Assert(units, gocheck.DeepEquals, expected)
	cont, err := getContainer("9930c24f1c4f")
	c.Assert(err, gocheck.IsNil)
	c.Assert(cont.IP, gocheck.Equals, "127.0.0.1")
	c.Assert(cont.HostPort, gocheck.Equals, "9024")
	if sshHandler.requests[0].URL.Path == "/container/127.0.0.4" {
		sshHandler.requests[0], sshHandler.requests[1] = sshHandler.requests[1], sshHandler.requests[0]
	}
	c.Assert(sshHandler.requests[0].URL.Path, gocheck.Equals, "/container/127.0.0.3")
	c.Assert(sshHandler.requests[0].Method, gocheck.Equals, "DELETE")
	c.Assert(sshHandler.requests[1].URL.Path, gocheck.Equals, "/container/127.0.0.4")
	c.Assert(sshHandler.requests[1].Method, gocheck.Equals, "DELETE")
	c.Assert(rtesting.FakeRouter.HasRoute("make-up", "http://127.0.0.1:9025"), gocheck.Equals, false)
	c.Assert(rtesting.FakeRouter.HasRoute("make-up", "http://127.0.0.1:9024"), gocheck.Equals, true)
	c.Assert(calls, gocheck.Equals, 2)
}

func (s *S) TestProvisionCollectStatusEmpty(c *gocheck.C) {
	s.conn.Collection(s.collName).RemoveAll(nil)
	output := map[string][][]byte{"ps -q": {[]byte("")}}
	fexec := &etesting.FakeExecutor{Output: output}
	setExecut(fexec)
	defer setExecut(nil)
	var p dockerProvisioner
	units, err := p.CollectStatus()
	c.Assert(err, gocheck.IsNil)
	c.Assert(units, gocheck.HasLen, 0)
}

func (s *S) TestProvisionCollection(c *gocheck.C) {
	collection := collection()
	c.Assert(collection.Name, gocheck.Equals, s.collName)
}

func (s *S) TestProvisionSetCName(c *gocheck.C) {
	var p dockerProvisioner
	app := testing.NewFakeApp("myapp", "python", 1)
	rtesting.FakeRouter.AddBackend("myapp")
	rtesting.FakeRouter.AddRoute("myapp", "127.0.0.1")
	cname := "mycname.com"
	err := p.SetCName(app, cname)
	c.Assert(err, gocheck.IsNil)
	c.Assert(rtesting.FakeRouter.HasBackend(cname), gocheck.Equals, true)
	c.Assert(rtesting.FakeRouter.HasRoute(cname, "127.0.0.1"), gocheck.Equals, true)
}

func (s *S) TestProvisionUnsetCName(c *gocheck.C) {
	var p dockerProvisioner
	app := testing.NewFakeApp("myapp", "python", 1)
	rtesting.FakeRouter.AddBackend("myapp")
	rtesting.FakeRouter.AddRoute("myapp", "127.0.0.1")
	cname := "mycname.com"
	err := p.SetCName(app, cname)
	c.Assert(err, gocheck.IsNil)
	c.Assert(rtesting.FakeRouter.HasBackend(cname), gocheck.Equals, true)
	c.Assert(rtesting.FakeRouter.HasRoute(cname, "127.0.0.1"), gocheck.Equals, true)
	err = p.UnsetCName(app, cname)
	c.Assert(rtesting.FakeRouter.HasBackend(cname), gocheck.Equals, false)
	c.Assert(rtesting.FakeRouter.HasRoute(cname, "127.0.0.1"), gocheck.Equals, false)
}

func (s *S) TestProvisionerIsCNameManager(c *gocheck.C) {
	var _ provision.CNameManager = &dockerProvisioner{}
}

func (s *S) TestCommands(c *gocheck.C) {
	var p dockerProvisioner
	expected := []cmd.Command{
		addNodeToSchedulerCmd{},
		removeNodeFromSchedulerCmd{},
		listNodesInTheSchedulerCmd{},
		&sshAgentCmd{},
	}
	c.Assert(p.Commands(), gocheck.DeepEquals, expected)
}

func (s *S) TestProvisionerIsCommandable(c *gocheck.C) {
	var _ provision.Commandable = &dockerProvisioner{}
}

func (s *S) TestSwap(c *gocheck.C) {
	var p dockerProvisioner
	app1 := testing.NewFakeApp("app1", "python", 1)
	app2 := testing.NewFakeApp("app2", "python", 1)
	rtesting.FakeRouter.AddBackend(app1.GetName())
	rtesting.FakeRouter.AddRoute(app1.GetName(), "127.0.0.1")
	rtesting.FakeRouter.AddBackend(app2.GetName())
	rtesting.FakeRouter.AddRoute(app2.GetName(), "127.0.0.2")
	err := p.Swap(app1, app2)
	c.Assert(err, gocheck.IsNil)
	c.Assert(rtesting.FakeRouter.HasBackend(app1.GetName()), gocheck.Equals, true)
	c.Assert(rtesting.FakeRouter.HasBackend(app2.GetName()), gocheck.Equals, true)
	c.Assert(rtesting.FakeRouter.HasRoute(app2.GetName(), "127.0.0.1"), gocheck.Equals, true)
	c.Assert(rtesting.FakeRouter.HasRoute(app1.GetName(), "127.0.0.2"), gocheck.Equals, true)
}

func (s *S) TestExecuteCommandOnce(c *gocheck.C) {
	var handler FakeSSHServer
	handler.output = ". .."
	server := httptest.NewServer(&handler)
	defer server.Close()
	host, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	portNumber, _ := strconv.Atoi(port)
	config.Set("docker:ssh-agent-port", portNumber)
	defer config.Unset("docker:ssh-agent-port")
	app := testing.NewFakeApp("almah", "static", 1)
	p := dockerProvisioner{}
	container := container{ID: "c-036", AppName: app.GetName(), Type: "python", IP: "10.10.10.1", HostAddr: host}
	err := s.conn.Collection(s.collName).Insert(container)
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Collection(s.collName).Remove(bson.M{"_id": container.ID})
	var stdout, stderr bytes.Buffer
	err = p.ExecuteCommandOnce(&stdout, &stderr, app, "ls", "-lh")
	c.Assert(err, gocheck.IsNil)
	c.Assert(err, gocheck.IsNil)
	c.Assert(stderr.Bytes(), gocheck.IsNil)
	c.Assert(stdout.String(), gocheck.Equals, ". ..")
	body := handler.bodies[0]
	input := cmdInput{Cmd: "ls", Args: []string{"-lh"}}
	c.Assert(body, gocheck.DeepEquals, input)
}

func (s *S) TestExecuteCommandOnceWithoutContainers(c *gocheck.C) {
	app := testing.NewFakeApp("almah", "static", 2)
	p := dockerProvisioner{}
	var stdout, stderr bytes.Buffer
	err := p.ExecuteCommandOnce(&stdout, &stderr, app, "ls", "-lh")
	c.Assert(err, gocheck.Not(gocheck.IsNil))
}
