package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bucloud/hwapi"
)

var (
	start    time.Time = time.Now().UTC().Add(-time.Hour * 24)
	end      time.Time = time.Now().UTC()
	worker   uint      = 1
	hosthash string    = ""
	logtype  string    = "cds"
	user     string    = ""
	password string    = ""
	output   string    = "./"
)

func init() {
	s := flag.String("s", time.Now().UTC().Add(-time.Hour*24).Format(time.RFC3339), "download log from time, RFC3339 format is supported")
	e := flag.String("e", time.Now().UTC().Format(time.RFC3339), "download log till time, RFC3339 format is supported")
	flag.StringVar(&user, "u", user, "username to access log server")
	flag.StringVar(&password, "p", password, "password to access log server")
	flag.StringVar(&hosthash, "host", hosthash, "set hosthash")
	flag.StringVar(&logtype, "t", logtype, "set logtype, available value cds,cdi")
	flag.StringVar(&output, "d", output, "set directory to store logfiles")
	flag.UintVar(&worker, "n", worker, "set workers")
	flag.Parse()

	if st, e1 := time.Parse("2006-01-02T15:04:05Z", *s); *s != "" && e1 == nil {
		start = st
	}

	if et, e2 := time.Parse("2006-01-02T15:04:05Z", *e); *e != "" && e2 == nil {
		end = et
	}

	if user == "" || password == "" {
		fmt.Printf("user/password are required\n")
		flag.Usage()
		os.Exit(3)
	}

}

func main() {
	api := hwapi.Init(&http.Transport{MaxConnsPerHost: 20})
	if at, e := api.Auth(user, password, true); e != nil {
		fmt.Printf("get access token failed, %s\n", e.Error())
		os.Exit(1)
	} else {
		api.AuthToken = at
	}
	if strings.Contains(hosthash, ".") {
		// try search hosthash by hostname
		if _, e := api.Auth(user, password); e != nil {
			fmt.Printf("get API accesstoken failed when try translate hostname to hashcode, %s\n", e.Error())
			os.Exit(1)
		}
		cu, e := api.AboutMe()
		if e != nil {
			fmt.Printf("get current user info failed, %s\n", e.Error())
			os.Exit(2)
		}
		r, e := api.Search(cu.AccountHash, hosthash, 10)
		if e != nil {
			fmt.Printf("search hosthash failed, %s\n", e.Error())
		}
		hh := r.Hostnames
		switch len(hh) {
		case 1:
			hosthash = hh[0].HostHash
		case 0:
			fmt.Printf("hostname not found under %s(%s)\n", cu.AccountHash, user)
			os.Exit(2)
		default:
			fmt.Printf("found more then one hosthash, please pick one of them\n")
			for _, h := range hh {
				fmt.Printf("%12s  ->   %s\n", h.HostHash, h.Name)
			}
			os.Exit(1)
		}

	}
	files, e := api.SearchLogs(hosthash, logtype, start, end)
	if e != nil {
		fmt.Printf("search logs failed, %s\n", e.Error())
		os.Exit(1)
	}
	api.SetDownloadConcurrency(worker)
	fmt.Printf("going to donwload logs, there are %d files need to download\n", len(files))
	if _, e := api.Downloads(output, files...); e != nil {
		fmt.Printf("%s", e.Error())
	}
	fmt.Printf("Download completed.\n")
}
