#!/usr/bin/env bash
VERSION=0.0.0
docker build . -f Dockerfile -t freeswitch_fax:${VERSION}
