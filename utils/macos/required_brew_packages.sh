#!/bin/bash

#
# Copyright 2021 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
#space-seperated packages to install
brew_packages="chipmk/tap/docker-mac-net-connect coreutils"

#space-seperated list of services to start after installation
start_services="chipmk/tap/docker-mac-net-connect"

launchDaemonsDir="/Library/LaunchDaemons"

if [[ ! "${OSTYPE}" =~ ^darwin.* ]]; then
  echo "must be running on a MacOS";
  exit 1;
fi

brew -v > /dev/null
brew_installed=$?
if [ ${brew_installed} -ne 0 ]; then
  echo "MacOS requires brew to install '${brew_packages}' and allow traffic from host machine to containers"
  exit 1
fi

for package in ${brew_packages}; do
    brew list ${package} > /dev/null || brew install ${package}
done;

for service in ${start_services}; do
  #all of this block is required to test if daemon is already configured
  #and reduce unnecessary sudo password prompts
  serviceParts=(${service//\// })
  servicePart=${serviceParts[${#serviceParts[@]} - 1]}
  file=$(ls ${launchDaemonsDir} | grep "${servicePart}")
  if [[ ! "${file}" == "" ]]; then
    #check daemon is configured to runatload and keepalive is true, if so we assume it is already running
    cat ${launchDaemonsDir}"/"${file} | grep RunAtLoad -A1 | grep true > /dev/null
    not_running=$?
    #configured to runatload, now confirm it is configured to keepalive
    if [ $not_running -eq 0 ]; then
      cat ${launchDaemonsDir}"/"${file} | grep KeepAlive -A1 | grep true > /dev/null
      not_running=$?
    fi
  else
    not_running=1
  fi

  #service is not configured to start, so let's run the sudo command now
  if [ ${not_running} -ne 0 ]; then
    sudo brew services start $service
  fi
done;
