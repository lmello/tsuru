// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package service

import (
	"encoding/json"
	stderrors "errors"
	"github.com/globocom/tsuru/app/bind"
	"github.com/globocom/tsuru/auth"
	"github.com/globocom/tsuru/db"
	"github.com/globocom/tsuru/errors"
	"github.com/globocom/tsuru/log"
	"github.com/globocom/tsuru/rec"
	"labix.org/v2/mgo/bson"
	"net/http"
	"regexp"
)

var (
	ErrServiceInstanceNotFound = stderrors.New("Service instance not found")
	ErrInvalidInstanceName     = stderrors.New("Invalid service instance name")
	ErrAccessNotAllowed        = stderrors.New("User does not have access to this service instance")

	instanceNameRegexp = regexp.MustCompile(`^[A-Za-z][-a-zA-Z0-9_]+$`)
)

type ServiceInstance struct {
	Name        string
	ServiceName string `bson:"service_name"`
	Apps        []string
	Teams       []string
}

// DeleteInstance deletes the service instance from the database.
func DeleteInstance(si *ServiceInstance) error {
	if len(si.Apps) > 0 {
		msg := "This service instance is bound to at least one app. Unbind them before removing it"
		return stderrors.New(msg)
	}
	endpoint, err := si.Service().getClient("production")
	if err == nil {
		endpoint.Destroy(si)
	}
	conn, err := db.Conn()
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.ServiceInstances().Remove(bson.M{"name": si.Name})
}

// MarshalJSON marshals the ServiceName in json format.
func (si *ServiceInstance) MarshalJSON() ([]byte, error) {
	info, err := si.Info()
	if err != nil {
		info = nil
	}
	data := map[string]interface{}{
		"Name":        si.Name,
		"Teams":       si.Teams,
		"Apps":        si.Apps,
		"ServiceName": si.ServiceName,
		"Info":        info,
	}
	return json.Marshal(&data)
}

func (si *ServiceInstance) Info() (map[string]string, error) {
	endpoint, err := si.Service().getClient("production")
	if err != nil {
		return nil, stderrors.New("endpoint does not exists")
	}
	result, err := endpoint.Info(si)
	if err != nil {
		return nil, err
	}
	info := map[string]string{}
	for _, d := range result {
		info[d["label"]] = d["value"]
	}
	return info, nil
}

func (si *ServiceInstance) Create() error {
	conn, err := db.Conn()
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.ServiceInstances().Insert(si)
}

func (si *ServiceInstance) Service() *Service {
	conn, err := db.Conn()
	if err != nil {
		log.Printf("Failed to connect to the database: %s", err)
		return nil
	}
	defer conn.Close()
	var s Service
	conn.Services().Find(bson.M{"_id": si.ServiceName}).One(&s)
	return &s
}

func (si *ServiceInstance) AddApp(appName string) error {
	index := si.FindApp(appName)
	if index > -1 {
		return stderrors.New("This instance already has this app.")
	}
	si.Apps = append(si.Apps, appName)
	return nil
}

func (si *ServiceInstance) FindApp(appName string) int {
	index := -1
	for i, name := range si.Apps {
		if name == appName {
			index = i
			break
		}
	}
	return index
}

func (si *ServiceInstance) RemoveApp(appName string) error {
	index := si.FindApp(appName)
	if index < 0 {
		return stderrors.New("This app is not bound to this service instance.")
	}
	copy(si.Apps[index:], si.Apps[index+1:])
	si.Apps = si.Apps[:len(si.Apps)-1]
	return nil
}

func (si *ServiceInstance) update() error {
	conn, err := db.Conn()
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.ServiceInstances().Update(bson.M{"name": si.Name}, si)
}

// BindApp makes the bind between the service instance and an app.
func (si *ServiceInstance) BindApp(app bind.App) error {
	err := si.AddApp(app.GetName())
	if err != nil {
		return &errors.HTTP{Code: http.StatusConflict, Message: "This app is already bound to this service instance."}
	}
	err = si.update()
	if err != nil {
		return err
	}
	if len(app.GetUnits()) == 0 {
		return &errors.HTTP{Code: http.StatusPreconditionFailed, Message: "This app does not have an IP yet."}
	}
	envsChan := make(chan map[string]string, len(app.GetUnits())+1)
	errChan := make(chan error, len(app.GetUnits())+1)
	for _, unit := range app.GetUnits() {
		go func(unit bind.Unit) {
			vars, err := si.BindUnit(app, unit)
			if err != nil {
				errChan <- err
				return
			}
			envsChan <- vars
		}(unit)
	}
	var envVars []bind.EnvVar
	select {
	case envs := <-envsChan:
		for k, v := range envs {
			envVars = append(envVars, bind.EnvVar{
				Name:         k,
				Value:        v,
				Public:       false,
				InstanceName: si.Name,
			})
		}
		return app.SetEnvs(envVars, false)
	case err = <-errChan:
	}
	return err
}

// BindUnit makes the bind between the binder and an unit.
func (si *ServiceInstance) BindUnit(app bind.App, unit bind.Unit) (map[string]string, error) {
	endpoint, err := si.Service().getClient("production")
	if err != nil {
		return nil, err
	}
	return endpoint.Bind(si, app, unit)
}

// UnbindApp makes the unbind between the service instance and an app.
func (si *ServiceInstance) UnbindApp(app bind.App) error {
	err := si.RemoveApp(app.GetName())
	if err != nil {
		return &errors.HTTP{Code: http.StatusPreconditionFailed, Message: "This app is not bound to this service instance."}
	}
	err = si.update()
	if err != nil {
		return err
	}
	for _, unit := range app.GetUnits() {
		go func(unit bind.Unit) {
			si.UnbindUnit(unit)
		}(unit)
	}
	var envVars []string
	for k := range app.InstanceEnv(si.Name) {
		envVars = append(envVars, k)
	}
	return app.UnsetEnvs(envVars, false)
}

// UnbindUnit makes the unbind between the service instance and an unit.
func (si *ServiceInstance) UnbindUnit(unit bind.Unit) error {
	endpoint, err := si.Service().getClient("production")
	if err != nil {
		return err
	}
	return endpoint.Unbind(si, unit)
}

// Status returns the service instance status.
func (si *ServiceInstance) Status() (string, error) {
	endpoint, err := si.Service().getClient("production")
	if err != nil {
		return "", err
	}
	return endpoint.Status(si)
}

func genericServiceInstancesFilter(services interface{}, teams []string) (q, f bson.M) {
	f = bson.M{"name": 1, "service_name": 1, "apps": 1}
	q = bson.M{}
	if len(teams) != 0 {
		q["teams"] = bson.M{"$in": teams}
	}
	if v, ok := services.([]Service); ok {
		names := GetServicesNames(v)
		q["service_name"] = bson.M{"$in": names}
	}
	if v, ok := services.(Service); ok {
		q["service_name"] = v.Name
	}
	return
}

func CreateServiceInstance(name string, service *Service, user *auth.User) error {
	if !instanceNameRegexp.MatchString(name) {
		return ErrInvalidInstanceName
	}
	instance := ServiceInstance{
		Name:        name,
		ServiceName: service.Name,
	}
	teams, err := user.Teams()
	if err != nil {
		return err
	}
	instance.Teams = make([]string, 0, len(teams))
	for _, team := range teams {
		if service.HasTeam(&team) || !service.IsRestricted {
			instance.Teams = append(instance.Teams, team.Name)
		}
	}
	endpoint, err := service.getClient("production")
	if err != nil {
		return err
	}
	err = endpoint.Create(&instance)
	if err != nil {
		return err
	}
	conn, err := db.Conn()
	if err != nil {
		endpoint.Destroy(&instance)
		return err
	}
	defer conn.Close()
	err = conn.ServiceInstances().Insert(instance)
	if err != nil {
		endpoint.Destroy(&instance)
		return err
	}
	return nil
}

func GetServiceInstancesByServices(services []Service) ([]ServiceInstance, error) {
	var instances []ServiceInstance
	conn, err := db.Conn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	q, _ := genericServiceInstancesFilter(services, []string{})
	f := bson.M{"name": 1, "service_name": 1}
	err = conn.ServiceInstances().Find(q).Select(f).All(&instances)
	return instances, err
}

func GetServiceInstancesByServicesAndTeams(services []Service, u *auth.User) ([]ServiceInstance, error) {
	var instances []ServiceInstance
	teams, err := u.Teams()
	if err != nil {
		return nil, err
	}
	if len(teams) == 0 {
		return nil, nil
	}
	conn, err := db.Conn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	q, f := genericServiceInstancesFilter(services, auth.GetTeamsNames(teams))
	err = conn.ServiceInstances().Find(q).Select(f).All(&instances)
	return instances, err
}

func GetServiceInstance(name string, u *auth.User) (*ServiceInstance, error) {
	conn, err := db.Conn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rec.Log(u.Email, "get-service-instance", name)
	var instance ServiceInstance
	err = conn.ServiceInstances().Find(bson.M{"name": name}).One(&instance)
	if err != nil {
		return nil, ErrServiceInstanceNotFound
	}
	if !auth.CheckUserAccess(instance.Teams, u) {
		return nil, ErrAccessNotAllowed
	}
	return &instance, nil
}
