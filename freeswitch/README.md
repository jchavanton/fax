/usr/local/freeswitch/bin/fs_cli -x "show detailed_bridged_calls as json" | jq .
sofia profile external siptrace on
docker exec freeswitch1 /usr/local/freeswitch/bin/fs_cli -x "originate sofia/external/fax@15.222.241.45:5062 &txfax(/files/txfax.tiff)"
