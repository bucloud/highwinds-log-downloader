# highwinds-log-downloader

highlands raw logs download tool

# support

1. use http mode access logs _only availabe before 2020-01-01_
2. both hosthash and hostname are available in host flag, if more then one hostnames found, suggestion printed
3. multiple process supported, but with a max 100
4. support both privateKey and HMacKey to access raw logs stored in GCS, prefer use privateKey
5. support automatic generate privateKey or HMacKey _note, in order to decrease useless keys, key would not generate if there are three keys exists_
6. if the environment variable GOOGLE_APPLICATION_CREDENTIALS is set, the application will treat as privateKey provided
7. if the environment AWS_ACCESS_KEY and AWS_SECRET_KEY is set, the application will treat as HMacKey provided
8. automatic generate credential data would store under `${homepath}/.highwinds/hcs.ini`

# usage

    go build -o logdownloader
    ./logdownloader -u asd -p asd -host a1b1c1d1 -n 3
