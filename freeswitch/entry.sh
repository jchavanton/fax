#!/bin/bash -e

ulimit -c unlimited
COREDIR=/tmp
if [ -d $COREDIR ]; then
         echo "ERROR: core dump directory $COREDIR does not exist."
fi
echo "$COREDIR/core.%e.sig%s.%p" > /proc/sys/kernel/core_pattern

if [ -z "${PUBLIC_IP}" ]; then
	PUBLIC_IP=$(curl ifconfig.me)
fi
export PUBLIC_IP=${PUBLIC_IP}
LOCAL_IPV4=$(hostname -I | cut -d' ' -f1)
export LOCAL_IPV4=${LOCAL_IPV4}
export ESL_PORT=${ESL_PORT}
export SIP_PORT_EXTERNAL=${SIP_PORT_EXTERNAL}
export TLS_PORT_EXTERNAL=${TLS_PORT_EXTERNAL}
export SIP_PORT_INTERNAL=${SIP_PORT_INTERNAL}
export TLS_PORT_INTERNAL=${TLS_PORT_INTERNAL}
export RTP_START_PORT=${RTP_START_PORT}
export RTP_END_PORT=${RTP_END_PORT}

dockerize -timeout 120s -template /usr/local/freeswitch/conf/autoload_configs/event_socket.conf.xml.tmpl:/usr/local/freeswitch/conf/autoload_configs/event_socket.conf.xml \
	 -template /usr/local/freeswitch/conf/vars.xml.tmpl:/usr/local/freeswitch/conf/vars.xml \
	 -template /usr/local/freeswitch/conf/sip_profiles/external.xml.tmpl:/usr/local/freeswitch/conf/sip_profiles/external.xml \
	 -template /usr/local/freeswitch/conf/sip_profiles/internal.xml.tmpl:/usr/local/freeswitch/conf/sip_profiles/internal.xml \
	 -template /usr/local/freeswitch/conf/autoload_configs/switch.conf.xml.tmpl:/usr/local/freeswitch/conf/autoload_configs/switch.conf.xml

# CMD="tail -f /dev/null"
if [ "$1" = "" ]; then
	CMD="stdbuf -i0 -o0 -e0 /usr/local/freeswitch/bin/freeswitch -c -nonat"
else
	CMD="$*"
fi
echo "Running [$CMD]"
exec $CMD
echo "exiting ..."
