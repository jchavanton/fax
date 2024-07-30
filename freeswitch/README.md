/usr/local/freeswitch/bin/fs_cli  -x "show detailed_bridged_calls as json" | jq .

docker  exec freeswitch1 /usr/local/freeswitch/bin/fs_cli -x "originate sofia/external/fax@15.222.241.45 &txfax(/files/txfax.tiff)"

https://github.com/PasifikTelekom/How-to-Receive-Fax-using-FreeSWITCH


docker exec freeswitch1 /usr/local/freeswitch/bin/fs_cli -x "originate {absolute_codec_string='PCMU'}sofia/external/fax@15.222.241.45:5062 &txfax(/files/txfax2.tiff)"
