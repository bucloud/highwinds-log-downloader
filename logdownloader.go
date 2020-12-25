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
	start         time.Time = time.Now().UTC().Add(-time.Hour * 24)
	end           time.Time = time.Now().UTC()
	forceGenerate bool      = false
	keyLimit      int       = 3
	worker        uint      = 1
	hosthashs     string    = ""
	hostPattern   string    = ""
	logtype       string    = "cds"
	user          string    = ""
	password      string    = ""
	output        string    = "./"
	showSecret    bool      = false

	// Cfg configure
	Cfg nsConfigure = make(nsConfigure)
)

func init() {
	s := flag.String("s", time.Now().UTC().Add(-time.Hour*24).Format(time.RFC3339), "download log from time, RFC3339 format is supported")
	e := flag.String("e", time.Now().UTC().Format(time.RFC3339), "download log till time, RFC3339 format is supported")
	flag.StringVar(&hosthashs, "host", hosthashs, "set hosthash, use comma to split multiple hosthash")
	flag.StringVar(&hostPattern, "pattern", hostPattern, "use host pattern as host, this will download all logs for host match pattern, Note, only support wildcard")
	flag.StringVar(&logtype, "t", logtype, "set logtype, available value cds,cdi")
	flag.StringVar(&output, "d", output, "set directory to store logfiles")
	flag.UintVar(&worker, "n", worker, "set workers")
	flag.BoolVar(&showSecret, "showSecret", showSecret, "show secert data instead of hide them")
	flag.Parse()

	if st, e1 := time.Parse("2006-01-02T15:04:05Z", *s); *s != "" && e1 == nil {
		start = st
	}

	if et, e2 := time.Parse("2006-01-02T15:04:05Z", *e); *e != "" && e2 == nil {
		end = et
	}

	if hosthashs == "" && hostPattern == "" {
		fmt.Printf("host must provided\n")
		os.Exit(1)
	}

	var err error
	Cfg, err = loadConfig()
	if len(os.Args) == 2 && os.Args[1] == "config" {
		Cfg = Cfg.editConfig()
		if err := Cfg.save(); err != nil {
			fmt.Printf("edit configure failed, %s\n", err.Error())
		}
		os.Exit(0)
	} else {
		if err != nil {
			fmt.Printf("load configure failed, %s\n", err.Error())
			os.Exit(2)
		}
	}
}

func main() {
	conf := Cfg[defaultConfigScope]
	if conf == nil {
		fmt.Printf("default/global configure not found\n")
		os.Exit(2)
	}
	api := hwapi.Init(&http.Transport{MaxConnsPerHost: 20})
	if conf.AuthType == "token" {
		api.SetToken(conf.Token)
	} else {
		if _, e := api.Auth(conf.Username, conf.Password); e != nil {
			fmt.Println(e.Error())
			os.Exit(1)
		}
	}
	cu, e := api.AboutMe()
	if e != nil {
		fmt.Printf("get account info faild, %s\n", e.Error())
		os.Exit(1)
	}
	hosts := []*hwapi.HostName{}
	if hostPattern != "" {
		r, e := api.Search(cu.AccountHash, hostPattern, 10)
		if e != nil {
			fmt.Printf("search host failed, %s\n", e.Error())
		}
		hosts = append(hosts, r.Hostnames...)
	} else {
		for _, hosthash := range strings.Split(hosthashs, ",") {
			// force search host
			r, e := api.Search(cu.AccountHash, hosthash, 10)
			if e != nil {
				fmt.Printf("search host failed, %s\n", e.Error())
				os.Exit(2)
			}
			hh := r.Hostnames
			switch len(hh) {
			case 1:
				hosts = append(hosts, r.Hostnames...)
			case 0:
				fmt.Printf("hostname not found under %s(%s)\n", cu.AccountHash, user)
				os.Exit(2)
			default:
				hosthash = scanInput{
					Placeholder: "found more then one hosthash, please pick one of them",
					Default:     hh[0].HostHash,
					Options: func(list []*hwapi.HostName) []*inputOptions {
						res := []*inputOptions{}
						for _, h := range list {
							res = append(res, &inputOptions{
								Label: h.Name,
								Value: h.HostHash,
							})
						}
						return res
					}(hh),
				}.scan()
			}
			hosts = append(hosts, func(list []*hwapi.HostName, hosthash string) *hwapi.HostName {
				for _, h := range list {
					if h.HostHash == hosthash {
						return h
					}
				}
				return nil
			}(hh, hosthash))
		}
	}

	// begin search log files

	for _, h := range hosts {

	}
}
