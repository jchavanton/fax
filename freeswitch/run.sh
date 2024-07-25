#!/bin/bash
VERSION=0.0.0
DIR_PREFIX=`pwd`
IMAGE=freeswitch

docker ps | grep "${IMAGE}1" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker kill || /usr/bin/true;
docker container ls -a | grep "${IMAGE}1" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker container rm || /usr/bin/true;

docker ps | grep "${IMAGE}2" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker kill || /usr/bin/true;
docker container ls -a | grep "${IMAGE}2" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker container rm || /usr/bin/true;

docker ps | grep "${IMAGE}3" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker kill || /usr/bin/true;
docker container ls -a | grep "${IMAGE}3" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker container rm || /usr/bin/true;

docker run -d --net=host \
              --privileged \
              --name=${IMAGE}1 \
              -e ESL_PORT=8021 \
              -e SIP_PORT_EXTERNAL=5510 \
              -e TLS_PORT_EXTERNAL=5511 \
              -e SIP_PORT_INTERNAL=7010 \
              -e TLS_PORT_INTERNAL=7011 \
              -e RTP_START_PORT=10000 \
              -e RTP_END_PORT=12999 \
              --log-driver syslog \
              --log-opt tag="{{.Name}}" \
              --restart unless-stopped \
              -v ${DIR_PREFIX}/../files:/files \
              ${IMAGE}:${VERSION}

