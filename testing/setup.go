// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testing provide test helpers for various actions.
package testing

import (
	"github.com/globocom/config"
	"launchpad.net/goamz/iam/iamtest"
	"launchpad.net/goamz/s3/s3test"
	"launchpad.net/gocheck"
	"net/http"
	"net/http/httptest"
)

type T struct {
	Admin        user
	AdminTeam    team
	S3Server     *s3test.Server
	IamServer    *iamtest.Server
	GitAPIServer string
	GitRWHost    string
	GitROHost    string
}

type user struct {
	Email    string
	Password string
}

type team struct {
	Name  string `bson:"_id"`
	Users []string
}

func (t *T) StartAmzS3AndIAM(c *gocheck.C) {
	var err error
	t.S3Server, err = s3test.NewServer(&s3test.Config{Send409Conflict: true})
	c.Assert(err, gocheck.IsNil)
	config.Set("aws:s3:endpoint", t.S3Server.URL())
	t.IamServer, err = iamtest.NewServer()
	c.Assert(err, gocheck.IsNil)
	config.Set("aws:iam:endpoint", t.IamServer.URL())
	config.Unset("aws:s3:bucketEndpoint")
}

func (t *T) SetGitConfs(c *gocheck.C) {
	t.GitAPIServer, _ = config.GetString("git:api-server")
	t.GitROHost, _ = config.GetString("git:ro-host")
	t.GitRWHost, _ = config.GetString("git:rw-host")
}

func (t *T) RollbackGitConfs(c *gocheck.C) {
	config.Set("git:api-server", t.GitAPIServer)
	config.Set("git:ro-host", t.GitROHost)
	config.Set("git:rw-host", t.GitRWHost)
}

// starts a new httptest.Server and returns it
// Also changes git:host, git:port and git:protocol to match the server's url
func (t *T) StartGandalfTestServer(h http.Handler) *httptest.Server {
	ts := httptest.NewServer(h)
	config.Set("git:api-server", ts.URL)
	return ts
}
