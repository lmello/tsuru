package main

import (
	"bytes"
	"github.com/timeredbull/tsuru/cmd"
	. "launchpad.net/gocheck"
	"net/http"
	"strings"
)

func (s *S) TestServiceList(c *C) {
	output := `[{"service": "mysql", "instances": ["mysql01", "mysql02"]}, {"service": "oracle", "instances": []}]`
	expectedPrefix := `+---------+------------------+
| Service | Instances        |`
	lineMysql := "| mysql   | mysql01, mysql02 |"
	lineOracle := "| oracle  |                  |"
	ctx := cmd.Context{
		Cmds:   []string{},
		Args:   []string{},
		Stdout: manager.Stdout,
		Stderr: manager.Stderr,
	}
	trans := &conditionalTransport{
		transport{
			msg:    output,
			status: http.StatusOK,
		},
		func(req *http.Request) bool {
			return req.URL.Path == "/services/instances"
		},
	}
	client := cmd.NewClient(&http.Client{Transport: trans})
	err := (&ServiceList{}).Run(&ctx, client)
	c.Assert(err, IsNil)
	table := manager.Stdout.(*bytes.Buffer).String()
	c.Assert(table, Matches, "^"+expectedPrefix+".*")
	c.Assert(table, Matches, "^.*"+lineMysql+".*")
	c.Assert(table, Matches, "^.*"+lineOracle+".*")
}

func (s *S) TestServiceListWithEmptyResponse(c *C) {
	output := "[]"
	expected := ""
	ctx := cmd.Context{
		Cmds:   []string{},
		Args:   []string{},
		Stdout: manager.Stdout,
		Stderr: manager.Stderr,
	}
	trans := &conditionalTransport{
		transport{
			msg:    output,
			status: http.StatusOK,
		},
		func(req *http.Request) bool {
			return req.URL.Path == "/services/instances"
		},
	}
	client := cmd.NewClient(&http.Client{Transport: trans})
	err := (&ServiceList{}).Run(&ctx, client)
	c.Assert(err, IsNil)
	c.Assert(manager.Stdout.(*bytes.Buffer).String(), Equals, expected)
}

func (s *S) TestInfoServiceList(c *C) {
	expected := &cmd.Info{
		Name:    "service-list",
		Usage:   "service-list",
		Desc:    "Get all available services, and user's instances for this services",
		MinArgs: 0,
	}
	command := &ServiceList{}
	c.Assert(command.Info(), DeepEquals, expected)
}

func (s *S) TestServiceListShouldBeInfoer(c *C) {
	var infoer cmd.Infoer
	c.Assert(&ServiceList{}, Implements, &infoer)
}

func (s *S) TestServiceListShouldBeCommand(c *C) {
	var command cmd.Command
	c.Assert(&ServiceList{}, Implements, &command)
}

func (s *S) TestServiceBind(c *C) {
	var called bool
	ctx := cmd.Context{
		Cmds:   []string{},
		Args:   []string{"my-mysql", "g1"},
		Stdout: manager.Stdout,
		Stderr: manager.Stderr,
	}
	trans := &conditionalTransport{
		transport{
			msg:    "",
			status: http.StatusOK,
		},
		func(req *http.Request) bool {
			called = true
			return req.Method == "PUT" && req.URL.Path == "/services/instances/my-mysql/g1"
		},
	}
	client := cmd.NewClient(&http.Client{Transport: trans})
	err := (&ServiceBind{}).Run(&ctx, client)
	c.Assert(err, IsNil)
	c.Assert(called, Equals, true)
	c.Assert(manager.Stdout.(*bytes.Buffer).String(), Equals, "Instance my-mysql successfully binded to the app g1.\n")
}

func (s *S) TestServiceBindWithRequestFailure(c *C) {
	ctx := cmd.Context{
		Cmds:   []string{},
		Args:   []string{"my-mysql", "g1"},
		Stdout: manager.Stdout,
		Stderr: manager.Stderr,
	}
	trans := &transport{
		msg:    "This user does not have access to this app.",
		status: http.StatusForbidden,
	}
	client := cmd.NewClient(&http.Client{Transport: trans})
	err := (&ServiceBind{}).Run(&ctx, client)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, trans.msg)
}

func (s *S) TestServiceBindInfo(c *C) {
	expected := &cmd.Info{
		Name:    "bind",
		Usage:   "bind <instancename> <appname>",
		Desc:    "bind a service instance to an app",
		MinArgs: 2,
	}
	c.Assert((&ServiceBind{}).Info(), DeepEquals, expected)
}

func (s *S) TestServiceBindIsAnInfoer(c *C) {
	var infoer cmd.Infoer
	c.Assert(&ServiceBind{}, Implements, &infoer)
}

func (s *S) TestServiceBindIsACommand(c *C) {
	var command cmd.Command
	c.Assert(&ServiceBind{}, Implements, &command)
}

func (s *S) TestServiceUnbind(c *C) {
	var called bool
	ctx := cmd.Context{
		Cmds:   []string{},
		Args:   []string{"hand", "pocket"},
		Stdout: manager.Stdout,
		Stderr: manager.Stderr,
	}
	trans := &conditionalTransport{
		transport{
			msg:    "",
			status: http.StatusOK,
		},
		func(req *http.Request) bool {
			called = true
			return req.Method == "DELETE" && req.URL.Path == "/services/instances/hand/pocket"
		},
	}
	client := cmd.NewClient(&http.Client{Transport: trans})
	err := (&ServiceUnbind{}).Run(&ctx, client)
	c.Assert(err, IsNil)
	c.Assert(called, Equals, true)
	c.Assert(manager.Stdout.(*bytes.Buffer).String(), Equals, "Instance hand successfully unbinded from the app pocket.\n")
}

func (s *S) TestServiceUnbindWithRequestFailure(c *C) {
	ctx := cmd.Context{
		Cmds:   []string{},
		Args:   []string{"hand", "pocket"},
		Stdout: manager.Stdout,
		Stderr: manager.Stderr,
	}
	trans := &transport{
		msg:    "This app is not binded to this service.",
		status: http.StatusPreconditionFailed,
	}
	client := cmd.NewClient(&http.Client{Transport: trans})
	err := (&ServiceUnbind{}).Run(&ctx, client)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, trans.msg)
}

func (s *S) TestServiceUnbindInfo(c *C) {
	expected := &cmd.Info{
		Name:    "unbind",
		Usage:   "unbind <instancename> <appname>",
		Desc:    "unbind a service instance from an app",
		MinArgs: 2,
	}
	c.Assert((&ServiceUnbind{}).Info(), DeepEquals, expected)
}

func (s *S) TestServiceUnbindIsAnInfoer(c *C) {
	var infoer cmd.Infoer
	c.Assert(&ServiceUnbind{}, Implements, &infoer)
}

func (s *S) TestServiceUnbindIsAComand(c *C) {
	var command cmd.Command
	c.Assert(&ServiceUnbind{}, Implements, &command)
}

func (s *S) TestServiceAddInfo(c *C) {
	usage := `service-add <servicename> <serviceinstancename>
e.g.:

    $ tsuru service-add mongodb tsuru_mongodb

Will add a new instance of the "mongodb" service, named "tsuru_mongodb".`
	expected := &cmd.Info{
		Name:    "service-add",
		Usage:   usage,
		Desc:    "Create a service instance to one or more apps make use of.",
		MinArgs: 2,
	}
	command := &ServiceAdd{}
	c.Assert(command.Info(), DeepEquals, expected)
}

func (s *S) TestServiceAddRun(c *C) {
	result := "service successfully added.\n"
	args := []string{
		"my_app_db",
		"mysql",
	}
	context := cmd.Context{
		Cmds:   []string{},
		Args:   args,
		Stdout: manager.Stdout,
		Stderr: manager.Stderr,
	}
	client := cmd.NewClient(&http.Client{Transport: &transport{msg: result, status: http.StatusOK}})
	err := (&ServiceAdd{}).Run(&context, client)
	c.Assert(err, IsNil)
	obtained := manager.Stdout.(*bytes.Buffer).String()
	c.Assert(obtained, Equals, result)
}

func (s *S) TestServiceInstanceStatusInfo(c *C) {
	usg := `service-status <serviceinstancename>
e.g.:

    $ tsuru service-status my_mongodb
`
	expected := &cmd.Info{
		Name:    "service-status",
		Usage:   usg,
		Desc:    "Check status of a given service instance.",
		MinArgs: 1,
	}
	got := (&ServiceInstanceStatus{}).Info()
	c.Assert(got, DeepEquals, expected)
}

func (s *S) TestServiceInstanceStatusRun(c *C) {
	result := `Service instance "foo" is up`
	args := []string{"fooBar"}
	context := cmd.Context{
		Cmds:   []string{},
		Args:   args,
		Stdout: manager.Stdout,
		Stderr: manager.Stderr,
	}
	client := cmd.NewClient(&http.Client{Transport: &transport{msg: result, status: http.StatusOK}})
	err := (&ServiceInstanceStatus{}).Run(&context, client)
	c.Assert(err, IsNil)
	obtained := manager.Stdout.(*bytes.Buffer).String()
	obtained = strings.Replace(obtained, "\n", "", -1)
	c.Assert(obtained, Equals, result)
}

func (s *S) TestServiceInfoInfo(c *C) {
	usg := `service-info <service>
e.g.:

    $ tsuru service-info mongodb
`
	expected := &cmd.Info{
		Name:    "service-info",
		Usage:   usg,
		Desc:    "List all instances of a service",
		MinArgs: 1,
	}
	got := (&ServiceInfo{}).Info()
	c.Assert(got, DeepEquals, expected)
}

func (s *S) TestServiceInfoRun(c *C) {
	result := `[{"Name":"mymongo", "Apps":["myapp"]}]`
	expected := `Info for "mongodb"
+-----------+-------+
| Instances | Apps  |
+-----------+-------+
| mymongo   | myapp |
+-----------+-------+
`
	args := []string{"mongodb"}
	context := cmd.Context{
		Cmds:   []string{},
		Args:   args,
		Stdout: manager.Stdout,
		Stderr: manager.Stderr,
	}
	client := cmd.NewClient(&http.Client{Transport: &transport{msg: result, status: http.StatusOK}})
	err := (&ServiceInfo{}).Run(&context, client)
	c.Assert(err, IsNil)
	obtained := manager.Stdout.(*bytes.Buffer).String()
	c.Assert(obtained, Equals, expected)
}

func (s *S) TestServiceDocInfo(c *C) {
	i := (&ServiceDoc{}).Info()
	expected := &cmd.Info{
		Name:    "service-doc",
		Usage:   "service-doc <servicename>",
		Desc:    "Show documentation of a service",
		MinArgs: 1,
	}
	c.Assert(i, DeepEquals, expected)
}

func (s *S) TestServiceDocRun(c *C) {
	result := `This is a test doc for a test service.
Service test is foo bar.
`
	expected := result
	ctx := cmd.Context{
		Cmds:   []string{},
		Args:   []string{"foo"},
		Stdout: manager.Stdout,
		Stderr: manager.Stderr,
	}
	client := cmd.NewClient(&http.Client{Transport: &transport{msg: result, status: http.StatusOK}})
	err := (&ServiceDoc{}).Run(&ctx, client)
	c.Assert(err, IsNil)
	obtained := manager.Stdout.(*bytes.Buffer).String()
	c.Assert(obtained, Equals, expected)
}

func (s *S) TestServiceRemoveInfo(c *C) {
	i := (&ServiceRemove{}).Info()
	expected := &cmd.Info{
		Name:    "service-remove",
		Usage:   "service-remove <serviceinstancename>",
		Desc:    "Removes a service instance",
		MinArgs: 1,
	}
	c.Assert(i, DeepEquals, expected)
}

func (s *S) TestServiceRemoveRun(c *C) {
	ctx := cmd.Context{
		Cmds:   []string{},
		Args:   []string{"some-service-instance"},
		Stdout: manager.Stdout,
		Stderr: manager.Stderr,
	}
	result := "service instance successfuly removed"
	client := cmd.NewClient(&http.Client{Transport: &transport{msg: result, status: http.StatusOK}})
	err := (&ServiceRemove{}).Run(&ctx, client)
	c.Assert(err, IsNil)
	obtained := manager.Stdout.(*bytes.Buffer).String()
	c.Assert(obtained, Equals, result+"\n")
}