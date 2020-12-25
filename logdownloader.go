package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/bucloud/hwapi"
	"gopkg.in/ini.v1"
)

var (
	start                  time.Time = time.Now().UTC().Add(-time.Hour * 24)
	end                    time.Time = time.Now().UTC()
	maxResult              int       = 10
	forceGenerate          bool      = false
	keyLimit               int       = 3
	worker                 uint      = 1
	hosthashs              string    = ""
	hostPattern            string    = ""
	logtype                string    = "cds"
	user                   string    = ""
	password               string    = ""
	output                 string    = "./"
	showSecret             bool      = false
	autoGenerateCredential bool      = false

	// Cfg configure
	Cfg     nsConfigure   = make(nsConfigure)
	urlChan chan []string = make(chan []string, 3000)
)

func init() {
	s := flag.String("s", time.Now().UTC().Add(-time.Hour*24).Format(time.RFC3339), "download log from time, RFC3339 format is supported")
	e := flag.String("e", time.Now().UTC().Format(time.RFC3339), "download log till time, RFC3339 format is supported")
	flag.StringVar(&hosthashs, "host", hosthashs, "set hosthash, use comma to split multiple hosthash")
	flag.StringVar(&hostPattern, "pattern", hostPattern, "use host pattern as host, this will download all logs for host match pattern, Note, only support wildcard")
	flag.StringVar(&logtype, "t", logtype, "set logtype, available value cds,cdi")
	flag.StringVar(&output, "d", output, "set directory to store logfiles, support local and AWS s3, use {remoteConfigName}:{prefix} when use AWS s3 as destination")
	flag.UintVar(&worker, "n", worker, "set workers")
	flag.IntVar(&maxResult, "max", maxResult, "set max search results")
	flag.BoolVar(&showSecret, "show_secret", showSecret, "show secert data instead of hide them")
	flag.BoolVar(&autoGenerateCredential, "auto", autoGenerateCredential, "auto generate credential(access_key_id,secret_key), note credential will not generated when there are 3 credentials already exists")
	flag.BoolVar(&forceGenerate, "force_generate", forceGenerate, "force generate credentials if there are 3 credentials already exists in account")
	flag.Parse()

	if st, e1 := time.Parse("2006-01-02T15:04:05Z", *s); *s != "" && e1 == nil {
		start = st
	}

	if et, e2 := time.Parse("2006-01-02T15:04:05Z", *e); *e != "" && e2 == nil {
		end = et
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
			os.Exit(3)
		}
	}

	if hosthashs == "" && hostPattern == "" {
		fmt.Printf("host must provided\n")
		os.Exit(1)
	}

}

func main() {
	conf := Cfg[ini.DefaultSection]
	if conf == nil {
		fmt.Printf("default/global configure not found\n")
		os.Exit(3)
	}
	api := hwapi.Init(&http.Transport{MaxConnsPerHost: 20})
	if strings.Index(output, ":") > 0 {
		remoteName := output[:strings.Index(output, ":")]
		remotePath := output[strings.Index(output, ":")+1:]
		if Cfg["remote-"+remoteName] == nil {
			fmt.Printf("remote configure not found\n")
			os.Exit(5)
		}
		api.SetRemoteS3Conf(remoteName, &aws.Config{
			Region:      aws.String(Cfg["remote-"+remoteName].Region),
			Credentials: credentials.NewStaticCredentials(Cfg["remote-"+remoteName].AccessKeyID, Cfg["remote-"+remoteName].SecretAccessKey, ""),
		})
		output = remoteName + ":" + Cfg["remote-"+remoteName].BucketName + ":" + remotePath
	}
	api.SetWorkers(worker)
	if conf.AuthType == "token" {
		api.SetToken(conf.Token)
	} else {
		if _, e := api.Auth(conf.Username, conf.Password); e != nil {
			fmt.Println(e.Error())
			os.Exit(4)
		}
	}
	cu, e := api.AboutMe()
	if e != nil {
		fmt.Printf("get account info faild, %s\n", e.Error())
		os.Exit(2)
	}
	hosts := []*hwapi.HostName{}
	if hostPattern != "" {
		r, e := api.Search(cu.AccountHash, hostPattern, maxResult)
		if e != nil {
			fmt.Printf("search host failed, %s\n", e.Error())
		}
		hosts = append(hosts, r.Hostnames...)
	} else {
		for _, hosthash := range strings.Split(hosthashs, ",") {
			// force search host
			r, e := api.Search(cu.AccountHash, hosthash, maxResult)
			if e != nil {
				fmt.Printf("search host failed, %s\n", e.Error())
				os.Exit(1)
			}
			hh := r.Hostnames
			switch len(hh) {
			case 1:
				hosts = append(hosts, r.Hostnames...)
			case 0:
				fmt.Printf("host %s not found under %s(%s)\n", hosthash, cu.AccountHash, cu.AccountName)
				os.Exit(2)
			default:
				hosts = append(hosts, func(list []*hwapi.HostName, hosthash string) *hwapi.HostName {
					for _, h := range list {
						if h.HostHash == hosthash {
							return h
						}
					}
					return nil
				}(hh, scanInput{
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
				}.scan()))
			}
		}
	}
	// parse raw log urls
	// allurls := []string{}
	for _, h := range hosts {
		hcred := &hwapi.HCSCredentials{}
		if Cfg[h.AccountHash] == nil {
			Cfg[h.AccountHash] = Cfg[ini.DefaultSection]
			if h.AccountHash != cu.AccountHash {
				Cfg[h.AccountHash].AccessKeyID = ""
				Cfg[h.AccountHash].SecretAccessKey = ""
				Cfg[h.AccountHash].PrivateKeyJSON = ""
			}
			Cfg.save()
		}

		if (Cfg[h.AccountHash].AccessKeyID == "" || Cfg[h.AccountHash].SecretAccessKey == "") && Cfg[h.AccountHash].PrivateKeyJSON == "" && autoGenerateCredential {
			// get gcs account
			var serviceAccount *hwapi.GCSAccount
			sa, err := api.GetGCSAccounts(h.AccountHash)
			if err != nil || len(sa.List) == 0 {
				// try create gcs account
				if serviceAccount, err = api.CreateGCSAccount(h.AccountHash, "auto generate log account", "log_account"); err != nil {
					fmt.Printf("try create service_account under account %s failed, %s\n", h.AccountHash, err.Error())
					os.Exit(5)
				}
			} else {
				serviceAccount = sa.List[0]
			}
			// try generate HMAC_key
			hmacs, err := api.GetGCSHMacKeys(h.AccountHash, serviceAccount.ID)
			if err != nil || len(hmacs.List) <= keyLimit || (len(hmacs.List) > keyLimit && forceGenerate) {
				// try generate hmac_key
				hmac, err := api.CreateGCSHMacKey(h.AccountHash, serviceAccount.ID)
				if err != nil {
					fmt.Printf("try create hmac_key under account/service_account_name %s/%s failed, %s", h.AccountHash, serviceAccount.Name, err.Error())
					os.Exit(5)
				}
				Cfg[h.AccountHash] = &configure{}
				Cfg[h.AccountHash].AccessKeyID = hmac.AccessID
				hcred.AccessKeyID = hmac.AccessID
				Cfg[h.AccountHash].SecretAccessKey = hmac.Secret
				hcred.SecretKey = hmac.Secret
				Cfg.save()
			} else {
				fmt.Printf("hmac_key generate failed, try create it manually")
				os.Exit(5)
			}
		} else if (Cfg[h.AccountHash].AccessKeyID != "" && Cfg[h.AccountHash].SecretAccessKey != "") || Cfg[h.AccountHash].PrivateKeyJSON != "" {
			hcred.AccessKeyID = Cfg[h.AccountHash].AccessKeyID
			hcred.SecretKey = Cfg[h.AccountHash].SecretAccessKey
			hcred.PrivateKeyJSON = Cfg[h.AccountHash].PrivateKeyJSON
		} else {
			fmt.Printf("subAccounts's configure not found, please create new config scope named %s\n", h.AccountHash)
			os.Exit(3)
		}

		urls, err := api.SearchLogsV2(&hwapi.SearchLogsOptions{
			HostHash:       h.HostHash,
			AccountHash:    h.AccountHash,
			StartDate:      start,
			EndDate:        end,
			LogType:        logtype,
			HCSCredentials: hcred,
		})
		if err != nil {
			fmt.Printf("search logs faild, %s\n", err.Error())
			os.Exit(1)
		}
		fmt.Printf("search raw logs for host %s success, there are %d files, begin downloading...\n", h.Name, len(urls))
		if strings.LastIndex(output, ":") > 0 && output[strings.LastIndex(output, ":")+1:] == "/" {
			output = output + "/" + h.Name + "/"
		}
		if _, e := api.Downloads(output, urls...); e != nil {
			fmt.Printf("%s\n", e.Error())
		}
		fmt.Printf("Download completed.\n")
	}
}
