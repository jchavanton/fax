#!/bin/bash

INSTALL_PREFIX="/opt/"

declare -a hct_hosts=("HCT_CLIENT" "HCT_SERVER")

retreive_hct_config() {
	ROLE="fax"
	INSTALL_DIR="${INSTALL_PREFIX}/${ROLE}"
	for i in "${hct_hosts[@]}"
	do
		if [ "$1" != "all" ] && [ "$1" != "$i" ] ; then continue; fi
#			if [ "$1" == "HCT_CLIENT" ] ;then
				printf "\ndownloading to [$i]\n"
				scp $i:$INSTALL_DIR/run.sh .
				scp $i:$INSTALL_DIR/freeswitch/* freeswitch
#			fi
#			if [ "$1" == "HCT_SERVER" ] ;then
#				printf "\ndownloading to [$i]\n"
#				scp $i:$INSTALL_DIR/freeswitch/* freeswitch/
#				scp $i:$INSTALL_DIR/kamailio/* kamailio/
#				scp $i:$INSTALL_DIR/kamailio/config/* kamailio/config
#				scp $i:$INSTALL_DIR/kamailio/sqlite/* kamailio/sqlite
#				scp $i:$INSTALL_DIR/server/* server/
#			fi
		done
}

instruction() {
	printf  "\nYou can specify a host name :\n\n"
	for i in "${hct_hosts[@]}"
	do
		echo "./retreive.sh $i"
	done
}

TARGET=$1
if [ "${TARGET}" == "" ]
then
	instruction
	retreive_hct_config
	exit
fi

retreive_hct_config $1
