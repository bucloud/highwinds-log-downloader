# highwinds-log-downloader

highlands raw logs download tool

# Support

1. use http mode access logs _only availabe before 2021-01-01_
2. both hosthash and hostname are available in host flag, if more then one hostnames found, suggestion printed
3. want to download raw logs for multiple hosts, use comma to split them in host flag or use pattern flag instead
4. multiple process supported, in order to reduce load, maximum 5 \* PROCESS_NUM is suggested
5. support both privateKey and HMacKey to access raw logs stored in GCS, prefer use privateKey
6. support automatic generate privateKey or HMacKey _note, in order to decrease useless keys, keys will not generate if there are three keys exists_
7. automatic generate credential data would store under `${homepath}/.highwinds/hcs.ini`
8. want to download speical hosts's raw logs in loop, just speical a non zero value to loop flag

# Note

1. download state is used to reduce duplicate download, Note, this application doesn't check wether dest exists file, just check state info
1. Currently, download state are controled by `https://github.com/bucloud/hwapi`, it save state only after downloads, that means SIGINT, SIGTERM could cause save failed

# usage

    go build -o logdownloader
    # store user/password and necessary into config file
    ./logdownloader config
    ./logdownloader -u asd -p asd -host a1b1c1d1 -n 3
