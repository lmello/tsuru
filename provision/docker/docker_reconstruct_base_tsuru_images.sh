#!/bin/bash

# Abort execution if anything goes wrong
set -e

#TODO: read formulasPath from config file
formulasPath=/home/git/charms/precise

function create_tsuru_base {
    docker pull base
    # TODO: criar config para definir o repositório ubuntu da imagem
    BUILD_JOB=$(docker run -d -t base /bin/bash -c "
        /bin/sed -i 's/archive.ubuntu.com\/ubuntu/mirror.globo.com\/ubuntu\/archive/g' /etc/apt/sources.list &&\
        apt-get update &&\
        apt-get upgrade &&\
        useradd ubuntu &&\
        mkdir -p /var/lib/tsuru/hooks &&\
        chown -R ubuntu /var/lib/tsuru/hooks\
        ")
    docker attach $BUILD_JOB
    BUILD_IMG=$(docker commit $BUILD_JOB tsuru_base)
    echo "Created new image: $BUILD_IMG and saved as tsuru_base"
}

function install_via_git {
    formula=$1
    IMAGE=tsuru_base
    SRC=$formulasPath/$formula/hooks
    DST=/var/lib/tsuru/hooks
    CMD="/var/lib/tsuru/hooks/install"
    BUILD_JOB=$(docker run -d -t base /bin/bash -c "
        cd $DEST; curl -sL 
        ")
    docker attach $BUILD_JOB
    BUILD_IMG=$(docker commit $BUILD_JOB tsuru_base)
    echo "Created new image: $BUILD_IMG and saved as tsuru_base"
}

# copia hooks da applicacao específica e executa o install do hook
function install { 
    formula=$1
    IMAGE=tsuru_base
    SRC=$formulasPath/$formula/hooks
    DST=/var/lib/tsuru/hooks
    CMD="/var/lib/tsuru/hooks/install"
    cd $SRC; tar cf - . | docker run -i $IMAGE /bin/sh -c "(cd $DST && tar xBf -); $CMD"
}

#create_tsuru_base
install static

for formula in $(ls $formulasPath);do 
    echo install $formula
done

#BUILD_JOB=$(docker run -d -t base apt-get update)
#docker attach $BUILD_JOB
#BUILD_IMG=$(docker commit $BUILD_JOB tsuru_base)
