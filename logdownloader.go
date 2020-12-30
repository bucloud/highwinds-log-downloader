package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/bucloud/hwapi"
	"github.com/rs/zerolog"
)

var (
	start                  time.Time     = time.Now().UTC().Add(-time.Hour * 24)
	end                    time.Time     = time.Now().UTC()
	maxResult              int           = 10
	forceGenerate          bool          = false
	keyLimit               int           = 3
	worker                 int           = 1
	hosthashs              string        = ""
	hostPattern            string        = ""
	logtype                string        = "cds"
	user                   string        = ""
	password               string        = ""
	output                 string        = "./"
	showSecret             bool          = false
	autoGenerateCredential bool          = false
	loglevel               string        = "error"
	loopInterval           time.Duration = time.Minute * 0
	fixTime                bool          = true
	config                 string        = configFile

	// Cfg configure
	Cfg     nsConfigure   = make(nsConfigure)
	urlChan chan []string = make(chan []string, 3000)

	logger zerolog.Logger
)

func init() {
	s := flag.String("s", time.Now().UTC().Add(-time.Hour*24).Format(time.RFC3339), "download log from time, RFC3339 format is supported")
	e := flag.String("e", time.Now().UTC().Format(time.RFC3339), "download log till time, RFC3339 format is supported")
	flag.StringVar(&hosthashs, "host", hosthashs, "set hosthash, use comma to split multiple hosthash")
	flag.StringVar(&hostPattern, "pattern", hostPattern, "use host pattern as host, this will download all logs for host match pattern, Note, only support wildcard")
	flag.StringVar(&logtype, "t", logtype, "set logtype, available value cds,cdi")
	flag.StringVar(&output, "d", output, "set directory to store logfiles, support local and AWS s3, use {remoteConfigName}:{prefix} when use AWS s3 as destination")
	flag.StringVar(&loglevel, "log", loglevel, "set loglevel to print, [panic,fatal,error,warn,info,debug,trace] are available value")
	flag.StringVar(&config, "config", config, "use speicaled config file or config scope name")
	flag.IntVar(&worker, "n", worker, "set workers")
	flag.IntVar(&maxResult, "max", maxResult, "set max search results")
	flag.BoolVar(&showSecret, "show_secret", showSecret, "show secert data instead of hide them")
	flag.BoolVar(&autoGenerateCredential, "auto", autoGenerateCredential, "auto generate credential(access_key_id,secret_key), note credential will not generated when there are 3 credentials already exists")
	flag.BoolVar(&forceGenerate, "force_generate", forceGenerate, "force generate credentials if there are 3 credentials already exists in account")
	flag.DurationVar(&loopInterval, "loop", loopInterval, "loop download logs with a provided time range, zero means disable loop")
	flag.BoolVar(&fixTime, "fix_time", fixTime, "fix start/end time in loop download mode")
	flag.Parse()

	switch loglevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	}
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	output.FormatLevel = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
	}
	output.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf(" %s ", i)
	}
	output.FormatFieldName = func(i interface{}) string {
		return fmt.Sprintf("%s=", i)
	}
	output.FormatFieldValue = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("%s", i))
	}
	logger = zerolog.New(output).With().Timestamp().Logger()

	if st, e1 := time.Parse("2006-01-02T15:04:05Z", *s); *s != "" && e1 == nil {
		start = st
	}

	if et, e2 := time.Parse("2006-01-02T15:04:05Z", *e); *e != "" && e2 == nil {
		end = et
	}

	if _, e := os.Open(config); e == nil {
		configFile = config
		config = ""
	}
	var err error
	Cfg, err = loadConfig()
	if len(os.Args) == 2 && os.Args[1] == "config" {
		Cfg = Cfg.editConfig()
		if err := Cfg.save(); err != nil {
			logger.Error().Err(err).Msg("edit configure failed")
		}
		os.Exit(0)
	} else {
		if err != nil {
			logger.Error().Err(err).Msg("load configure failed")
			os.Exit(3)
		}
	}

	if hosthashs == "" && hostPattern == "" {
		logger.Fatal().Msg("host/pattern must provided")
		os.Exit(1)
	}

}

func main() {
	conf := Cfg.Default(config)
	if conf == nil {
		logger.Panic().Msg("default/global configure not found")
		os.Exit(3)
	}
	api := hwapi.Init(
		&http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   60 * time.Second,
				KeepAlive: 60 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxConnsPerHost:     20,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		&logger,
		&hwapi.LocalCacheConfig{FilePath: "./.state", MaxSize: 256 * 1024 * 1024},
		worker,
	)
	if strings.Index(output, ":") > 0 {
		remoteName := output[:strings.Index(output, ":")]
		remotePath := output[strings.Index(output, ":")+1:]
		if Cfg["remote-"+remoteName] == nil {
			logger.Fatal().Msgf("remote configure %s not found", remoteName)
			os.Exit(5)
		}
		api.SetRemoteS3Conf(remoteName, &aws.Config{
			Region:      aws.String(Cfg["remote-"+remoteName].Region),
			Credentials: credentials.NewStaticCredentials(Cfg["remote-"+remoteName].AccessKeyID, Cfg["remote-"+remoteName].SecretAccessKey, ""),
		})
		output = remoteName + ":" + Cfg["remote-"+remoteName].BucketName + ":" + remotePath
	}
	if conf.AuthType == "token" {
		api.SetToken(conf.Token)
	} else {
		if _, e := api.Auth(conf.Username, conf.Password); e != nil {
			logger.Error().Err(e).Msg("get accesstoken failed")
			os.Exit(4)
		}
	}
	cu, e := api.AboutMe()
	if e != nil {
		logger.Error().Err(e).Msg("get account info failed")
		os.Exit(2)
	}
	hosts := []*hwapi.HostName{}
	if hostPattern != "" {
		logger.Info().Str("account_hash", cu.AccountHash).Str("search_key", hostPattern).Msg("search hosts by pattern")
		r, e := api.Search(cu.AccountHash, hostPattern, maxResult)
		if e != nil {
			logger.Error().Err(e).Msg("search host failed")
		}
		hosts = append(hosts, r.Hostnames...)
	} else {
		for _, hosthash := range strings.Split(hosthashs, ",") {
			// force search host
			logger.Info().Str("account_hash", cu.AccountHash).Str("search_key", hosthash).Msg("search hosts by host")
			r, e := api.Search(cu.AccountHash, hosthash, maxResult)
			if e != nil {
				logger.Error().Err(e).Str("account_hash", cu.AccountHash).Str("search_key", hosthash).Msg("search hosts failed")
				os.Exit(1)
			}
			hh := r.Hostnames
			switch len(hh) {
			case 1:
				hosts = append(hosts, r.Hostnames...)
			case 0:
				logger.Fatal().Str("host_hash", hosthash).Str("account_hash", cu.AccountHash).Str("account_name", cu.AccountName).Msg("hosts not found")
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
	for {
		ts := time.Now()
		for _, h := range hosts {
			startTime := time.Now()
			hcred := &hwapi.HCSCredentials{}
			if Cfg[h.AccountHash] == nil {
				Cfg[h.AccountHash] = Cfg.Default(config)
				if h.AccountHash != cu.AccountHash {
					Cfg[h.AccountHash].AccessKeyID = ""
					Cfg[h.AccountHash].SecretAccessKey = ""
					Cfg[h.AccountHash].PrivateKeyJSON = ""
				}
				Cfg.save()
			}

			if (Cfg[h.AccountHash].AccessKeyID == "" || Cfg[h.AccountHash].SecretAccessKey == "") && Cfg[h.AccountHash].PrivateKeyJSON == "" && autoGenerateCredential {
				logger.Debug().Str("account_hash", h.AccountHash).Msg("try auto generate service_account")
				// get gcs account
				var serviceAccount *hwapi.GCSAccount
				sa, err := api.GetGCSAccounts(h.AccountHash)
				if err != nil || len(sa.List) == 0 {
					// try create gcs account
					if serviceAccount, err = api.CreateGCSAccount(h.AccountHash, "auto generate log account", "log_account"); err != nil {
						logger.Error().Err(err).Str("account_hash", h.AccountHash).Msg("create service_account failed")
						os.Exit(5)
					}
				} else {
					serviceAccount = sa.List[0]
				}
				logger.Debug().Str("account_hash", h.AccountHash).Msg("try auto generate hmac_keys")
				// try generate HMAC_key
				hmacs, err := api.GetGCSHMacKeys(h.AccountHash, serviceAccount.ID)
				if err != nil || len(hmacs.List) <= keyLimit || (len(hmacs.List) > keyLimit && forceGenerate) {
					// try generate hmac_key
					hmac, err := api.CreateGCSHMacKey(h.AccountHash, serviceAccount.ID)
					if err != nil {
						logger.Error().Err(err).Str("account_hash", h.AccountHash).Str("service_account_name", serviceAccount.Name).Msg("create service_account failed")
						os.Exit(5)
					}
					Cfg[h.AccountHash] = &configure{}
					Cfg[h.AccountHash].AccessKeyID = hmac.AccessID
					hcred.AccessKeyID = hmac.AccessID
					Cfg[h.AccountHash].SecretAccessKey = hmac.Secret
					hcred.SecretKey = hmac.Secret
					Cfg.save()
				} else {
					logger.Error().Msg("hmac_key generate failed, try create it manually")
					os.Exit(5)
				}
			} else if (Cfg[h.AccountHash].AccessKeyID != "" && Cfg[h.AccountHash].SecretAccessKey != "") || Cfg[h.AccountHash].PrivateKeyJSON != "" {
				hcred.AccessKeyID = Cfg[h.AccountHash].AccessKeyID
				hcred.SecretKey = Cfg[h.AccountHash].SecretAccessKey
				hcred.PrivateKeyJSON = Cfg[h.AccountHash].PrivateKeyJSON
			} else {
				logger.Error().Str("account_hash", h.AccountHash).Msg("subAccounts's configure not found, please create new config")
				os.Exit(3)
			}
			logger.Trace().Str("host_hash", h.HostHash).Time("from", start).Time("to", end).Str("type", logtype).Msg("begin search raw logs")
			urls, err := api.SearchLogsV2(&hwapi.SearchLogsOptions{
				HostHash:       h.HostHash,
				AccountHash:    h.AccountHash,
				StartDate:      start,
				EndDate:        end,
				LogType:        logtype,
				HCSCredentials: hcred,
			})
			if err != nil {
				logger.Error().Err(err).Str("host_hash", h.HostHash).Time("from", start).Time("to", end).Str("type", logtype).Msg("search logs failed")
				os.Exit(1)
			}
			if len(urls) == 0 {
				logger.Info().Str("host_hash", h.HostHash).Time("from", start).Time("to", end).Str("type", logtype).Msg("found nothing, handle next")
				continue
			}
			logger.Info().Str("host_hash", h.HostHash).Time("from", start).Time("to", end).Str("type", logtype).Int("file_number", len(urls)).Msg("search raw log succeed")
			tempDir := output
			if strings.LastIndex(output, ":") > 0 {
				tempDir = tempDir + "/" + h.Name + "/"
			}
			if _, e := api.Downloads(tempDir, urls...); e != nil {
				logger.Error().Err(e).Str("host_hash", h.HostHash).Time("from", start).Time("to", end).Str("type", logtype).Int("file_number", len(urls)).Msg("download logs failed")
			} else {
				logger.Info().Str("host_hash", h.HostHash).Time("from", start).Time("to", end).Str("type", logtype).Int("file_number", len(urls)).Dur("spent", time.Since(startTime)).Msg("download complete")
			}
		}
		if loopInterval == time.Minute*0 {
			break
		}
		if time.Since(ts) <= loopInterval {
			logger.Debug().Dur("sleep", loopInterval-time.Since(ts)).Msg("sleep awhile")
		}
		if fixTime {
			start = start.Add(loopInterval - time.Minute)
			end = end.Add(loopInterval)
		}
	}
}
