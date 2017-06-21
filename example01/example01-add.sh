#!/bin/sh

NS=$1
CONTID=$2

echo ${NS}
echo ${CONTID}

# https://stackoverflow.com/questions/31265993/docker-networking-namespace-not-visible-in-ip-netns-list
mkdir -p /var/run/netns/
ln -sfT ${NS} /var/run/netns/${CONTID}

# Create veth link
/sbin/ip link add v-eth1 type veth peer name v-peer1

# Add peer-1 to NS
/sbin/ip link set v-peer1 netns ${CONTID}

# Setup  IP Address of v-eth1
/sbin/ip addr add 10.200.1.1/24 dev v-eth1
/sbin/ip link set v-eth1 up

# Setup IP address of v-peer1
/sbin/ip netns exec ${CONTID} ip addr add 10.200.1.2/24 dev v-peer1
/sbin/ip netns exec ${CONTID} ip link set v-peer1 up
