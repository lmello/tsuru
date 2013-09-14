// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/globocom/config"
	"github.com/globocom/tsuru/cmd"
	"github.com/globocom/tsuru/provision"
	"github.com/globocom/tsuru/testing"
	"launchpad.net/gocheck"
	"os"
)

func (s *S) TestCommandsFromBaseManagerAreRegistered(c *gocheck.C) {
	baseManager := cmd.NewManager("tsr", "0.2.0", "", os.Stdout, os.Stderr, os.Stdin)
	manager := buildManager()
	for name, instance := range baseManager.Commands {
		command, ok := manager.Commands[name]
		c.Assert(ok, gocheck.Equals, true)
		c.Assert(command, gocheck.FitsTypeOf, instance)
	}
}

func (s *S) TestBuildManagerLoadsConfig(c *gocheck.C) {
	buildManager()
	// As defined in testdata/tsuru.conf.
	listen, err := config.GetString("listen")
	c.Assert(err, gocheck.IsNil)
	c.Assert(listen, gocheck.Equals, "0.0.0.0:8080")
}

func (s *S) TestAPICmdIsRegistered(c *gocheck.C) {
	manager := buildManager()
	api, ok := manager.Commands["api"]
	c.Assert(ok, gocheck.Equals, true)
	tsrApi, ok := api.(*tsrCommand)
	c.Assert(ok, gocheck.Equals, true)
	c.Assert(tsrApi.Command, gocheck.FitsTypeOf, &apiCmd{})
}

func (s *S) TestCollectorCmdIsRegistered(c *gocheck.C) {
	manager := buildManager()
	collector, ok := manager.Commands["collector"]
	c.Assert(ok, gocheck.Equals, true)
	tsrCollector, ok := collector.(*tsrCommand)
	c.Assert(ok, gocheck.Equals, true)
	c.Assert(tsrCollector.Command, gocheck.FitsTypeOf, &collectorCmd{})
}

func (s *S) TestTokenCmdIsRegistered(c *gocheck.C) {
	manager := buildManager()
	token, ok := manager.Commands["token"]
	c.Assert(ok, gocheck.Equals, true)
	tsrToken, ok := token.(*tsrCommand)
	c.Assert(ok, gocheck.Equals, true)
	c.Assert(tsrToken.Command, gocheck.FitsTypeOf, tokenCmd{})
}

func (s *S) TestShouldRegisterAllCommandsFromProvisioners(c *gocheck.C) {
	fp := testing.NewFakeProvisioner()
	p := testing.CommandableProvisioner{FakeProvisioner: *fp}
	provision.Register("comm", &p)
	manager := buildManager()
	fake, ok := manager.Commands["fake"]
	c.Assert(ok, gocheck.Equals, true)
	tsrFake, ok := fake.(*tsrCommand)
	c.Assert(ok, gocheck.Equals, true)
	c.Assert(tsrFake.Command, gocheck.FitsTypeOf, &testing.FakeCommand{})
}

func (s *S) TestHealerCmdIsRegistered(c *gocheck.C) {
	manager := buildManager()
	healer, ok := manager.Commands["healer"]
	c.Assert(ok, gocheck.Equals, true)
	tsrHealer, ok := healer.(*tsrCommand)
	c.Assert(ok, gocheck.Equals, true)
	c.Assert(tsrHealer.Command, gocheck.FitsTypeOf, &healerCmd{})
}
