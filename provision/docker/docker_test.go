// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"github.com/globocom/commandmocker"
	"github.com/globocom/config"
	"github.com/globocom/tsuru/fs/testing"
	"io/ioutil"
	"launchpad.net/gocheck"
	"os"
)

func (s *S) TestLXCCreate(c *gocheck.C) {
	config.Set("docker:authorized-key-path", "somepath")
	tmpdir, err := commandmocker.Add("sudo", "$*")
	c.Assert(err, gocheck.IsNil)
	defer commandmocker.Remove(tmpdir)
	container := container{name: "container"}
	err = container.create()
	c.Assert(err, gocheck.IsNil)
	c.Assert(commandmocker.Ran(tmpdir), gocheck.Equals, true)
	expected := "docker run -d base /bin/bash container somepath"
	c.Assert(commandmocker.Output(tmpdir), gocheck.Equals, expected)
}

func (s *S) TestLXCStart(c *gocheck.C) {
	tmpdir, err := commandmocker.Add("sudo", "$*")
	c.Assert(err, gocheck.IsNil)
	defer commandmocker.Remove(tmpdir)
	container := container{name: "container"}
	err = container.start()
	c.Assert(err, gocheck.IsNil)
	c.Assert(commandmocker.Ran(tmpdir), gocheck.Equals, true)
	expected := "docker start container"
	c.Assert(commandmocker.Output(tmpdir), gocheck.Equals, expected)
}

func (s *S) TestLXCStop(c *gocheck.C) {
	tmpdir, err := commandmocker.Add("sudo", "$*")
	c.Assert(err, gocheck.IsNil)
	defer commandmocker.Remove(tmpdir)
	container := container{name: "container"}
	err = container.stop()
	c.Assert(err, gocheck.IsNil)
	c.Assert(commandmocker.Ran(tmpdir), gocheck.Equals, true)
	expected := "docker stop container"
	c.Assert(commandmocker.Output(tmpdir), gocheck.Equals, expected)
}

func (s *S) TestLXCDestroy(c *gocheck.C) {
	tmpdir, err := commandmocker.Add("sudo", "$*")
	c.Assert(err, gocheck.IsNil)
	defer commandmocker.Remove(tmpdir)
	container := container{name: "container"}
	err = container.destroy()
	c.Assert(err, gocheck.IsNil)
	c.Assert(commandmocker.Ran(tmpdir), gocheck.Equals, true)
	expected := "docker rm container"
	c.Assert(commandmocker.Output(tmpdir), gocheck.Equals, expected)
}

func (s *S) TestContainerIP(c *gocheck.C) {
	config.Set("docker:ip-timeout", 10)
	file, _ := os.Open("testdata/dnsmasq.leases")
	data, err := ioutil.ReadAll(file)
	c.Assert(err, gocheck.IsNil)
	rfs := &testing.RecordingFs{FileContent: string(data)}
	fsystem = rfs
	defer func() {
		fsystem = nil
	}()
	f, _ := rfs.Open("/var/lib/misc/dnsmasq.leases")
	f.Write(data)
	f.Close()
	cont := container{name: "vm1"}
	c.Assert(cont.ip(), gocheck.Equals, "10.10.10.10")
	cont = container{name: "notfound"}
	c.Assert(cont.ip(), gocheck.Equals, "")
}