#!/bin/bash
set -e

# Set host-name and enable-dbus
sed -i -e "s@#enable-dbus=yes@enable-dbus=no@" -e "s@#host-name=foo@host-name=rachael@" /etc/avahi/avahi-daemon.conf

# Restart service
/etc/init.d/avahi-daemon restart

rachael &

#Set the root password as root if not set as an ENV variable
export PASSWD=${PASSWD:=root}
#Set the root password
echo "root:$PASSWD" | chpasswd
#Spawn dropbear
dropbear -E -F

