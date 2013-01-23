// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package juju

import (
	"github.com/globocom/tsuru/heal"
	"os/exec"
)

func init() {
	heal.Register("bootstrap", &BootstrapHealer{})
}

// BootstrapHealer is an implementation for the Healer interface. For more
// details on how a healer work, check the documentation of the heal package.
type BootstrapHealer struct{}

// getBootstrapMachine returns the bootstrap machine.
func getBootstrapMachine() machine {
	p := JujuProvisioner{}
	output, _ := p.getOutput()
	// for juju bootstrap machine always is the machine 0.
	return output.Machines[0]
}

// NeedsHeal returns true if the AgentState of bootstrap machine is "not-started".
func (h *BootstrapHealer) NeedsHeal() bool {
	bootstrapMachine := getBootstrapMachine()
	return bootstrapMachine.AgentState == "not-started"
}

// Heal executes the action for heal the bootstrap agent.
func (h *BootstrapHealer) Heal() error {
	if h.NeedsHeal() {
		bootstrapMachine := getBootstrapMachine()
		args := []string{
			"-o",
			"StrictHostKeyChecking no",
			"-q",
			"-l",
			"ubuntu",
			bootstrapMachine.IpAddress,
			"sudo",
			"restart",
			"juju-machine-agent",
		}
		cmd := exec.Command("ssh", args...)
		return cmd.Run()
	}
	return nil
}