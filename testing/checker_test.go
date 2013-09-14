// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"github.com/globocom/config"
	"github.com/globocom/tsuru/db"
	"launchpad.net/gocheck"
	"time"
)

type CheckerSuite struct{}

var _ = gocheck.Suite(CheckerSuite{})

func (CheckerSuite) SetUpSuite(c *gocheck.C) {
	config.Set("database:url", "localhost:27017")
	config.Set("database:name", "tsuru_testing_test")
	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	action := map[string]interface{}{
		"user":   "glenda@tsuru.io",
		"action": "run-command",
		"extra":  []interface{}{"rm", "-rf", "/"},
		"date":   time.Now(),
	}
	err = conn.UserActions().Insert(action)
	c.Assert(err, gocheck.IsNil)
	actionNoDate := map[string]interface{}{
		"user":   "glenda@tsuru.io",
		"action": "list-apps",
		"extra":  nil,
		"date":   nil,
	}
	err = conn.UserActions().Insert(actionNoDate)
	c.Assert(err, gocheck.IsNil)
}

func (CheckerSuite) TearDownSuite(c *gocheck.C) {
	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	conn.Apps().Database.DropDatabase()
}

func (CheckerSuite) TestIsRecordedInfo(c *gocheck.C) {
	expected := &gocheck.CheckerInfo{
		Name:   "IsRecorded",
		Params: []string{"action"},
	}
	c.Assert(isRecordedChecker{}.Info(), gocheck.DeepEquals, expected)
}

func (CheckerSuite) TestIsRecordedCheckInvalidParameter(c *gocheck.C) {
	result, error := isRecordedChecker{}.Check([]interface{}{"action"}, []string{"action"})
	c.Assert(result, gocheck.Equals, false)
	c.Assert(error, gocheck.Equals, "First parameter must be of type Action or *Action")
}

func (CheckerSuite) TestIsRecordedCheckWithValue(c *gocheck.C) {
	action := Action{
		User:   "glenda@tsuru.io",
		Action: "run-command",
		Extra:  []interface{}{"rm", "-rf", "/"},
	}
	result, error := isRecordedChecker{}.Check([]interface{}{action}, []string{})
	c.Assert(result, gocheck.Equals, true)
	c.Assert(error, gocheck.Equals, "")
}

func (CheckerSuite) TestIsRecordedCheckWithReference(c *gocheck.C) {
	action := Action{
		User:   "glenda@tsuru.io",
		Action: "run-command",
		Extra:  []interface{}{"rm", "-rf", "/"},
	}
	result, error := isRecordedChecker{}.Check([]interface{}{&action}, []string{})
	c.Assert(result, gocheck.Equals, true)
	c.Assert(error, gocheck.Equals, "")
}

func (CheckerSuite) TestIsRecordedNotInDatabase(c *gocheck.C) {
	action := Action{
		User:   "glenda@tsuru.io",
		Action: "run-command",
		Extra:  []interface{}{"rm", "-rf", "/home"},
	}
	result, error := isRecordedChecker{}.Check([]interface{}{action}, []string{})
	c.Assert(result, gocheck.Equals, false)
	c.Assert(error, gocheck.Equals, "Action not in the database")
}

func (CheckerSuite) TestIsRecordedWithoutDate(c *gocheck.C) {
	action := Action{
		User:   "glenda@tsuru.io",
		Action: "list-apps",
	}
	result, error := isRecordedChecker{}.Check([]interface{}{action}, []string{})
	c.Assert(result, gocheck.Equals, false)
	c.Assert(error, gocheck.Equals, "Action was not recorded using rec.Log")
}
