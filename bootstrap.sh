#!/bin/sh

mkdir -p /tmp/tailscale
/var/runtime/tailscaled --tun=userspace-networking --socks5-server=localhost:1055 &
/var/runtime/tailscale up --auth-key=${TAILSCALE_AUTHKEY} --hostname=aws-lambda-app
echo Tailscale started
ALL_PROXY=socks5://localhost:1055/ /var/runtime/my-app
