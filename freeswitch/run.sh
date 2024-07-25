#!/bin/bash
VERSION=0.0.0
DIR_PREFIX=`pwd`
IMAGE=freeswitch_fax

docker ps | grep "${IMAGE}1" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker kill || /usr/bin/true;
docker container ls -a | grep "${IMAGE}1" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker container rm || /usr/bin/true;

docker ps | grep "${IMAGE}2" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker kill || /usr/bin/true;
docker container ls -a | grep "${IMAGE}2" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker container rm || /usr/bin/true;

docker ps | grep "${IMAGE}3" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker kill || /usr/bin/true;
docker container ls -a | grep "${IMAGE}3" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker container rm || /usr/bin/true;

docker run -d --net=host \
              --privileged \
              --name=${IMAGE}1 \
              -e ESL_PORT=8041 \
              -e SIP_PORT_EXTERNAL=5540 \
              -e TLS_PORT_EXTERNAL=5541 \
              -e SIP_PORT_INTERNAL=7040 \
              -e TLS_PORT_INTERNAL=7041 \
              -e RTP_START_PORT=12000 \
              -e RTP_END_PORT=12999 \
              --log-driver syslog \
              --log-opt tag="{{.Name}}" \
              --restart unless-stopped \
              -v ${DIR_PREFIX}/../files:/files \
              ${IMAGE}:${VERSION}

