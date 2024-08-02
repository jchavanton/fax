#!/bin/bash
CURRENT_COMMIT=$1
CONTAINER="fax_controller"
VERSION="0.0.0"


docker build . -f Dockerfile -t ${CONTAINER}:${VERSION}
