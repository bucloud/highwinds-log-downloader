# highwinds-log-downloader
highlands log download tool
# support
1. use http mode access logs
2. both hosthash and hostname are available in host flag, if more then one hostnames found, suggestion printed
3. multiple process supported, but with a max 100

# usage
    go build -o logdownloader
    ./logdownloader -u asd -p asd -host a1b1c1d1 -n 3
