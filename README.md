## how-to: manual operation

## PDF 2 TIFF
```
gs -q -dNOPAUSE -sDEVICE=tiffg4 -sOutputFile=files/tx.tiff files/T38_TEST_PAGES.pdf -c quit
gs -q -dNOPAUSE -sDEVICE=tiffg4 -sOutputFile=files/tx.tiff files/invoice.pdf -c quit
```

## TX from HCT_CLIENT
```
docker exec freeswitch1 /usr/local/freeswitch/bin/fs_cli -x "originate {absolute_codec_string='PCMU'}sofia/external/fax@15.222.241.45:5062 &txfax(/files/tx.tiff)"
```

## RX from HCT_SERVER
```
scp HCT_SERVER:/opt/fax/files/rx.tiff ./files/
```

## TIFF 2 PDF
```
tiff2pdf -o T38_TEST_PAGES_faxed.pdf -p A4 -F rx.tiff
```



https://developer.signalwire.com/freeswitch/FreeSWITCH-Explained/Modules/mod_spandsp_6587021/#fax
