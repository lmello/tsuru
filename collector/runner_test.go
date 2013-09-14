// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package collector

import (
	"github.com/globocom/tsuru/app"
	"github.com/globocom/tsuru/db"
	"github.com/globocom/tsuru/provision"
	"labix.org/v2/mgo/bson"
	"launchpad.net/gocheck"
	"time"
)

var pendingApps = []string{"as_i_rise", "the_infanta"}
var runningApps = []string{"caravan", "bu2b", "carnies"}

func createApp(conn *db.Storage, name, state string) {
	a := app.App{
		Name:  name,
		Units: []app.Unit{{State: state}},
	}
	err := conn.Apps().Insert(&a)
	if err != nil {
		panic(err)
	}
}

func createApps(conn *db.Storage) {
	for _, name := range pendingApps {
		createApp(conn, name, string(provision.StatusPending))
	}
	for _, name := range runningApps {
		createApp(conn, name, string(provision.StatusStarted))
	}
}

func destroyApps(conn *db.Storage) {
	allApps := append(pendingApps, runningApps...)
	conn.Apps().Remove(bson.M{"name": bson.M{"$in": allApps}})
}

func (s *S) TestJujuCollect(c *gocheck.C) {
	app1 := app.App{Name: "as_i_rise", Platform: "python"}
	app2 := app.App{Name: "the_infanta", Platform: "python"}
	s.provisioner.Provision(&app1)
	defer s.provisioner.Destroy(&app1)
	s.provisioner.Provision(&app2)
	defer s.provisioner.Destroy(&app2)
	createApps(s.conn)
	defer destroyApps(s.conn)
	ch := make(chan time.Time)
	go collect(ch)
	ch <- time.Now()
	close(ch)
	time.Sleep(1e9)
	var apps []app.App
	err := s.conn.Apps().Find(bson.M{"name": bson.M{"$in": []string{"as_i_rise", "the_infanta"}}}).Sort("name").All(&apps)
	c.Assert(err, gocheck.IsNil)
	c.Assert(apps[0].Units[1].Ip, gocheck.Equals, "10.10.10.1")
	c.Assert(apps[1].Units[1].Ip, gocheck.Equals, "10.10.10.2")
}
