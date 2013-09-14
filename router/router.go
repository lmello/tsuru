// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package router provides interfaces that need to be satisfied in order to
// implement a new router on tsuru.
package router

import (
	"fmt"
	"github.com/globocom/tsuru/db"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
)

var routers = make(map[string]Router)

// Register registers a new router.
func Register(name string, r Router) {
	routers[name] = r
}

// Get gets the named router from the registry.
func Get(name string) (Router, error) {
	r, ok := routers[name]
	if !ok {
		return nil, fmt.Errorf("Unknown router: %q.", name)
	}
	return r, nil
}

// Router is the basic interface of this package. It provides methods for
// managing backends and routes. Each backend can have multiple routes.
type Router interface {
	AddBackend(name string) error
	RemoveBackend(name string) error
	AddRoute(name, address string) error
	RemoveRoute(name, address string) error
	SetCName(cname, name string) error
	UnsetCName(cname, name string) error
	Addr(name string) (string, error)

	// Swap change the router between two backends.
	Swap(string, string) error

	// Routes returns a list of routes of a backend.
	Routes(name string) ([]string, error)
}

func collection() (*mgo.Collection, error) {
	conn, err := db.Conn()
	if err != nil {
		return nil, err
	}
	return conn.Collection("routers"), nil
}

// Store stores the app name related with the
// router name.
func Store(appName, routerName string) error {
	coll, err := collection()
	if err != nil {
		return err
	}
	data := map[string]string{
		"app":    appName,
		"router": routerName,
	}
	return coll.Insert(&data)
}

func Retrieve(appName string) (string, error) {
	coll, err := collection()
	if err != nil {
		return "", err
	}
	data := map[string]string{}
	err = coll.Find(bson.M{"app": appName}).One(&data)
	if err != nil {
		return "", err
	}
	return data["router"], nil
}

func Remove(appName string) error {
	coll, err := collection()
	if err != nil {
		return err
	}
	return coll.Remove(bson.M{"app": appName})
}

func swapBackendName(backend1, backend2 string) error {
	coll, err := collection()
	if err != nil {
		return err
	}
	router1, err := Retrieve(backend1)
	if err != nil {
		return err
	}
	router2, err := Retrieve(backend2)
	if err != nil {
		return err
	}
	update := bson.M{"$set": bson.M{"router": router2}}
	err = coll.Update(bson.M{"app": backend1}, update)
	if err != nil {
		return err
	}
	update = bson.M{"$set": bson.M{"router": router1}}
	err = coll.Update(bson.M{"app": backend2}, update)
	var result []interface{}
	coll.Find(nil).All(&result)
	return err
}

func Swap(r Router, backend1, backend2 string) error {
	routes1, err := r.Routes(backend1)
	if err != nil {
		return err
	}
	routes2, err := r.Routes(backend2)
	if err != nil {
		return err
	}
	for _, route := range routes1 {
		err = r.AddRoute(backend2, route)
		if err != nil {
			return err
		}
		err = r.RemoveRoute(backend1, route)
		if err != nil {
			return err
		}
	}
	for _, route := range routes2 {
		err = r.AddRoute(backend1, route)
		if err != nil {
			return err
		}
		err = r.RemoveRoute(backend2, route)
		if err != nil {
			return err
		}
	}
	return swapBackendName(backend1, backend2)
}
