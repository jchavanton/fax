#!/bin/bash

INSTALL_PREFIX="/opt/"

declare -a hct_hosts=("HCT_CLIENT" "HCT_SERVER")

deploy_fax_config() {
	ROLE="fax"
	INSTALL_DIR="${INSTALL_PREFIX}/${ROLE}"
	for i in "${hct_hosts[@]}"
	do
		if [ "$1" != "all" ] && [ "$1" != "$i" ] ; then continue; fi
		printf "\nuploading to [$i]\n"
		ssh $i "sudo mkdir -p $INSTALL_DIR && sudo chmod -R 777 $INSTALL_DIR \
		        && sudo mkdir -p ${INSTALL_DIR}/${ROLE} \
		        && sudo chmod -R 777 ${INSTALL_DIR}"
		if [ "$1" == "HCT_CLIENT" ] ;then
			ssh $i "sudo mkdir -p $INSTALL_DIR/files && sudo chmod -R 777 $INSTALL_DIR/files"
			scp README.md $i:$INSTALL_DIR/
			scp -r files/* $i:$INSTALL_DIR/files
			ssh $i "sudo mkdir -p $INSTALL_DIR/freeswitch && sudo chmod -R 777 $INSTALL_DIR/freeswitch"
			scp -r freeswitch/* $i:$INSTALL_DIR/freeswitch
			ssh $i "sudo mkdir -p $INSTALL_DIR/controller && sudo chmod -R 777 $INSTALL_DIR/controller"
			scp -r controller/* $i:$INSTALL_DIR/controller
		fi
		if [ "$1" == "HCT_SERVER" ] ;then
			scp README.md $i:$INSTALL_DIR/
			ssh $i "sudo mkdir -p $INSTALL_DIR/files && sudo chmod -R 777 $INSTALL_DIR/files"
			scp -r files/* $i:$INSTALL_DIR/files
			ssh $i "sudo mkdir -p $INSTALL_DIR/freeswitch && sudo chmod -R 777 $INSTALL_DIR/freeswitch"
			scp -r freeswitch/* $i:$INSTALL_DIR/freeswitch
		fi
		ssh $i "sudo chown -R root.root $INSTALL_DIR"
		done
}

instruction() {
	printf  "\nYou can specify a host name :\n\n"
	for i in "${hct_hosts[@]}"
	do
		echo "./deploy.sh $i"
	done
}

TARGET=$1
if [ "${TARGET}" == "" ]
then
	instruction
	deploy_fax_config 
	exit
fi

deploy_fax_config $1
