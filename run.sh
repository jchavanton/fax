docker exec freeswitch1 /usr/local/freeswitch/bin/fs_cli -x "originate {absolute_codec_string='PCMU'}sofia/external/fax@15.222.241.45:5062 &txfax(/files/invoice.tiff)"
