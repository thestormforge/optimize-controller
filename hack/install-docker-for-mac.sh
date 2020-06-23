#!/usr/bin/env bash

# refs:
# https://github.com/docker/for-mac/issues/2359#issuecomment-607154849

brew cask install docker --no-quarantine

sudo /bin/cp /Applications/Docker.app/Contents/Library/LaunchServices/com.docker.vmnetd /Library/PrivilegedHelperTools
sudo /bin/cp /Applications/Docker.app/Contents/Resources/com.docker.vmnetd.plist /Library/LaunchDaemons/
sudo /bin/chmod 544 /Library/PrivilegedHelperTools/com.docker.vmnetd
sudo /bin/chmod 644 /Library/LaunchDaemons/com.docker.vmnetd.plist
sudo /bin/launchctl load /Library/LaunchDaemons/com.docker.vmnetd.plist

/Applications/Docker.app/Contents/MacOS/Docker --unattended &>/dev/null &

retries=0
while ! docker info &>/dev/null ; do
    sleep 5
    retries=`expr ${retries} + 1`

    if pgrep -xq -- "Docker"; then
        echo 'Docker still running'
    else
        echo 'Docker not running, restart'
        /Applications/Docker.app/Contents/MacOS/Docker --unattended &>/dev/null &
    fi

    if [[ ${retries} -gt 30 ]]; then
        >&2 echo 'Failed to run Docker'
        exit 1
    fi;

    echo 'Waiting for Docker service to be in the running state'
done
