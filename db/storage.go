// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package db encapsulates tsuru connection with MongoDB.
//
// The function Open dials to MongoDB and returns a connection (represented by
// the Storage type). It manages an internal pool of connections, and
// reconnects in case of failures. That means that you should not store
// references to the connection, but always call Open.
package db

import (
	"fmt"
	"github.com/globocom/config"
	"labix.org/v2/mgo"
	"sync"
	"time"
)

var (
	conn        = make(map[string]*session) // pool of connections
	mut         sync.RWMutex                // for pool thread safety
	ticker      *time.Ticker                // for garbage collection
	maxIdleTime = 5 * time.Minute           // max idle time for connections
)

const period time.Duration = 7 * 24 * time.Hour

type session struct {
	s    *mgo.Session
	used time.Time
}

// Storage holds the connection with the database.
type Storage struct {
	session *mgo.Session
	dbname  string
}

func open(addr, dbname string) (*Storage, error) {
	sess, err := mgo.Dial(addr)
	if err != nil {
		return nil, err
	}
	copy := sess.Copy()
	go func(t time.Duration) {
		time.Sleep(t)
		copy.Close()
	}(maxIdleTime)
	storage := &Storage{session: copy, dbname: dbname}
	mut.Lock()
	conn[addr] = &session{s: sess, used: time.Now()}
	mut.Unlock()
	return storage, nil
}

// Open dials to the MongoDB database, and return the connection (represented
// by the type Storage).
//
// addr is a MongoDB connection URI, and dbname is the name of the database.
//
// This function returns a pointer to a Storage, or a non-nil error in case of
// any failure.
func Open(addr, dbname string) (storage *Storage, err error) {
	defer func() {
		if r := recover(); r != nil {
			storage, err = open(addr, dbname)
		}
	}()
	mut.RLock()
	if session, ok := conn[addr]; ok {
		mut.RUnlock()
		if err = session.s.Ping(); err == nil {
			mut.Lock()
			session.used = time.Now()
			conn[addr] = session
			mut.Unlock()
			copy := session.s.Copy()
			go func() {
				time.Sleep(maxIdleTime)
				copy.Close()
			}()
			return &Storage{copy, dbname}, nil
		}
		return open(addr, dbname)
	}
	mut.RUnlock()
	return open(addr, dbname)
}

// Conn reads the tsuru config and calls Open to get a database connection.
//
// Most tsuru packages should probably use this function. Open is intended for
// use when supporting more than one database.
func Conn() (*Storage, error) {
	url, err := config.GetString("database:url")
	if err != nil {
		return nil, fmt.Errorf("configuration error: %s", err)
	}
	dbname, err := config.GetString("database:name")
	if err != nil {
		return nil, fmt.Errorf("configuration error: %s", err)
	}
	return Open(url, dbname)
}

// Close closes the storage, releasing the connection.
func (s *Storage) Close() {
	s.session.Close()
}

// Collection returns a collection by its name.
//
// If the collection does not exist, MongoDB will create it.
func (s *Storage) Collection(name string) *mgo.Collection {
	return s.session.DB(s.dbname).C(name)
}

// Apps returns the apps collection from MongoDB.
func (s *Storage) Apps() *mgo.Collection {
	nameIndex := mgo.Index{Key: []string{"name"}, Unique: true}
	c := s.Collection("apps")
	c.EnsureIndex(nameIndex)
	return c
}

// Platforms returns the platforms collection from MongoDB.
func (s *Storage) Platforms() *mgo.Collection {
	return s.Collection("platforms")
}

// Logs returns the logs collection from MongoDB.
func (s *Storage) Logs() *mgo.Collection {
	appNameIndex := mgo.Index{Key: []string{"appname"}}
	sourceIndex := mgo.Index{Key: []string{"source"}}
	c := s.Collection("logs")
	c.EnsureIndex(appNameIndex)
	c.EnsureIndex(sourceIndex)
	return c
}

// Services returns the services collection from MongoDB.
func (s *Storage) Services() *mgo.Collection {
	c := s.Collection("services")
	return c
}

// ServiceInstances returns the services_instances collection from MongoDB.
func (s *Storage) ServiceInstances() *mgo.Collection {
	return s.Collection("service_instances")
}

// Users returns the users collection from MongoDB.
func (s *Storage) Users() *mgo.Collection {
	emailIndex := mgo.Index{Key: []string{"email"}, Unique: true}
	c := s.Collection("users")
	c.EnsureIndex(emailIndex)
	return c
}

func (s *Storage) Tokens() *mgo.Collection {
	return s.Collection("tokens")
}

func (s *Storage) PasswordTokens() *mgo.Collection {
	return s.Collection("password_tokens")
}

func (s *Storage) UserActions() *mgo.Collection {
	return s.Collection("user_actions")
}

// Teams returns the teams collection from MongoDB.
func (s *Storage) Teams() *mgo.Collection {
	return s.Collection("teams")
}

// Quota returns the quota collection from MongoDB.
func (s *Storage) Quota() *mgo.Collection {
	userIndex := mgo.Index{Key: []string{"owner"}, Unique: true}
	c := s.Collection("quota")
	c.EnsureIndex(userIndex)
	return c
}

func init() {
	ticker = time.NewTicker(time.Hour)
	go retire(ticker)
}

// retire retires old connections
func retire(t *time.Ticker) {
	for _ = range t.C {
		now := time.Now()
		var old []string
		mut.RLock()
		for k, v := range conn {
			if now.Sub(v.used) >= period {
				old = append(old, k)
			}
		}
		mut.RUnlock()
		mut.Lock()
		for _, c := range old {
			conn[c].s.Close()
			delete(conn, c)
		}
		mut.Unlock()
	}
}
