// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"errors"
	"launchpad.net/gocheck"
)

func (s *S) TestAppCreationError(c *gocheck.C) {
	e := AppCreationError{app: "myapp", Err: errors.New("failure in app")}
	expected := `Tsuru failed to create the app "myapp": failure in app`
	c.Assert(e.Error(), gocheck.Equals, expected)
}

func (s *S) TestNoTeamsError(c *gocheck.C) {
	e := NoTeamsError{}
	c.Assert(e.Error(), gocheck.Equals, "Cannot create app without teams.")
}
