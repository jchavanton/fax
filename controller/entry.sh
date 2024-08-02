#!/bin/bash

if [ "$1" = "" ]; then
	PORT=8090
	CMD="/main/main ${PORT}"
else
        CMD="$*"
fi

echo "Running [$CMD]"
exec $CMD
echo "exiting ..."
