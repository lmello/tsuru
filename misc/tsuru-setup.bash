#!/bin/bash -e

# Copyright 2013 tsuru authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# This script is responsible to install tsuru without any configuration
# if used standalone tsuru won't be able to startup because this
# script is provisioner independent.

function update_ubuntu() {
    echo "Updating and upgrading"
    sudo apt-get update
    sudo apt-get upgrade -y
}

function install_mongodb() {
    echo "Installing mongodb"
    sudo -E apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv 7F0CEB10
    sudo bash -c 'echo "deb http://downloads-distro.mongodb.org/repo/ubuntu-upstart dist 10gen" > /etc/apt/sources.list.d/10gen.list'
    sudo apt-get update -y
    sudo apt-get install mongodb-10gen -y --force-yes
}

function setup_platforms() {
    # this function should be called in the provisioner specific installation script
    # because mongo usually takes some time to startup, and it's not safe to call it from here
    # so call it after everything runs
    if [ ! -f platforms-setup.js ]; then
        curl -O https://raw.github.com/globocom/tsuru/master/misc/platforms-setup.js
    fi
    mongo tsuru platforms-setup.js
}

function install_beanstalkd() {
    echo "Installing beanstalkd"
    sudo apt-get install -y beanstalkd --force-yes
    sudo sed -i s/#START=yes/START=yes/ /etc/default/beanstalkd
    echo "starting beanstalkd"
    sudo service beanstalkd start
}

function install_tsuru() {
    echo "Downloading tsuru binary and copying to /usr/bin"
    curl -sL https://s3.amazonaws.com/tsuru/dist-server/tsr.tar.gz | sudo tar -xz -C /usr/bin
}

function main() {
    update_ubuntu
    install_mongodb
    install_beanstalkd
    install_tsuru
}

main
