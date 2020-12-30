package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/ini.v1"
)

type configure struct {
	AuthType        string `ini:"auth_type,omitempty" comment:"striketracker auth method, available options basic,token"`
	Username        string `ini:"user_name,omitempty" comment:"striketracker username"`
	Password        string `ini:"password,omitempty"`
	Token           string `ini:"token,omitempty"`
	PrivateKeyJSON  string `ini:"private_key_json,omitempty"`
	AccessKeyID     string `ini:"access_key_id,omitempty"`
	SecretAccessKey string `ini:"secret_access_key,omitempty"`
	BucketName      string `ini:"bucket_name,omitempty"`
	Region          string `ini:"region,omitempty" comment:"region"`
	Provider        string `ini:"provider,omitempty" comment:"remote storage service provider, only AWS S3 supported in remote configure"`
}

type nsConfigure map[string]*configure

var (
	// Cfg global variable contains configure data
	configFile string = homeDir(".highwinds", "hcs.ini")
)

func homeDir(s ...string) string {
	hp, err := os.UserHomeDir()
	if err != nil {
		hp = "."
	}
	return hp + "/" + strings.Join(s, "/")
}

func loadConfig() (nsConfigure, error) {
	var nc nsConfigure = make(nsConfigure)
	cfg, err := ini.Load(configFile)
	if err != nil {
		return nc, fmt.Errorf("read config failed, please run " + os.Args[0] + " config")
	}
	for _, section := range cfg.Sections() {
		c := configure{}
		if e := section.MapTo(&c); e != nil {
			return nc, fmt.Errorf("parse section %s error %s", section.Name(), e.Error())
		}
		nc[section.Name()] = &c
	}
	return nc, nil
}

// Default return default configure
func (nc nsConfigure) Default(scopename ...string) *configure {
	for n := range nc {
		if len(scopename) > 0 && n == scopename[0] {
			return nc[scopename[0]]
		}
	}
	return nc[ini.DefaultSection]
}
func (nc nsConfigure) save() error {
	os.Mkdir(configFile[:strings.LastIndex(configFile, "/")], 0700)
	f, _ := os.OpenFile(configFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	f.Close()
	cfg := ini.Empty()
	for k, c := range nc {
		cc, e := cfg.NewSection(k)
		if e != nil {
			return e
		}
		if e := cc.ReflectFrom(c); e != nil {
			return e
		}
	}
	if e := cfg.SaveTo(configFile); e != nil {
		return e
	}
	return nil
}

func (nc nsConfigure) printCurrentConfig() {
	fmt.Printf("# %-15s\t%-20s\t%-10s\t%-8s\t%-15s\t%-15s", "ConfigureScope", "Username", "Password", "Token", "PrivateKeyJSON", "AccessKey&Secret\n")
	i := 1
	for s, c := range nc {
		if strings.HasPrefix(s, "remote-") {
			fmt.Printf("%d %-15s\t%-20s\t%-10s\t%-8t\t%-15t\t%-15t\n", i, s, c.BucketName, c.Region, len(c.Token) > 1, len(c.PrivateKeyJSON) > 1, len(c.AccessKeyID) > 1 && len(c.SecretAccessKey) > 1)
		} else {
			fmt.Printf("%d %-15s\t%-20s\t%-10t\t%-8t\t%-15t\t%-15t\n", i, s, c.Username, len(c.Password) > 1, len(c.Token) > 1, len(c.PrivateKeyJSON) > 1, len(c.AccessKeyID) > 1 && len(c.SecretAccessKey) > 1)
		}
		i++
	}

}

func (nc nsConfigure) config() {
	nc.printCurrentConfig()
}
func (nc nsConfigure) editConfig() nsConfigure {
	// print selection
	for {
		// select method to access highwinds api
		option := scanInput{
			Placeholder: "Select option: ",
			Minlength:   1,
			Default:     "create",
			Options: []*inputOptions{
				&inputOptions{Value: "create", Label: "create new configure"},
				&inputOptions{Value: "print", Label: "print exists configure"},
				&inputOptions{Value: "edit", Label: "edit exists configure"},
				&inputOptions{Value: "delete", Label: "delete exists configure"},
				&inputOptions{Value: "quit", Label: "save & exit configure"},
			},
		}.scan()
		switch option {
		case "create":
			configScope := scanInput{
				Placeholder: "Select configure scope:",
				Minlength:   1,
				Default:     "default",
				Options: []*inputOptions{
					&inputOptions{Value: "default", Label: "default/global config"},
					&inputOptions{Value: "custom", Label: "set config for different account"},
					&inputOptions{Value: "remote", Label: "set remote config"},
				},
			}.scan()
			switch configScope {
			case "remote":
				configScope = scanInput{
					Placeholder: "Input remote config name such like s3-remote1 : ",
					Minlength:   2,
				}.scan()
				configScope = "remote-" + configScope
			case "custom":
				configScope = scanInput{
					Placeholder: "Input accountHash (in order to use configure more effectively, please use accountHash as scope name): ",
					Vaild:       func(s *string) (bool, error) { return len(*s) == 8, fmt.Errorf("sames not a vaild accountHash") },
				}.scan()
			case "default":
				configScope = ini.DefaultSection
			default:
				configScope = ini.DefaultSection
			}
			if nc[configScope] == nil {
				nc[configScope] = &configure{}
			}
			if strings.HasPrefix(configScope, "remote-") {
				nc[configScope].collectRemote()
			} else {
				if configScope != ini.DefaultSection && nc[ini.DefaultSection] != nil {
					nc[configScope].Username = nc[ini.DefaultSection].Username
					nc[configScope].Password = nc[ini.DefaultSection].Password
				}
				nc[configScope].collect()
			}
			continue
		case "edit":
			if len(nc) == 0 {
				fmt.Println("Nothing found, try create new configure")
				continue
			}
			// nc.printCurrentConfig()
			n := scanInput{
				Placeholder: "Which config do you want to edit?\n" + fmt.Sprintf("# %-15s\t%-20s\t%-10s\t%-8s\t%-15s\t%-15s", "ConfigureScope", "Username", "Password", "Token", "PrivateKeyJSON", "AccessKey&Secret"),
				Options: func(conf nsConfigure) []*inputOptions {
					var res []*inputOptions
					for sn, c := range conf {
						if strings.HasPrefix(sn, "remote-") {
							res = append(res, &inputOptions{
								Label: fmt.Sprintf("%-15s\t%-20s\t%-10s\t%-8t\t%-15t\t%-15t\n", sn, c.BucketName, c.Region, len(c.Token) > 1, len(c.PrivateKeyJSON) > 1, len(c.AccessKeyID) > 1 && len(c.SecretAccessKey) > 1),
								Value: sn,
							})
						} else {
							res = append(res, &inputOptions{
								Label: fmt.Sprintf("%-15s\t%-20s\t%-10t\t%-8t\t%-15t\t%-15t", sn, c.Username, len(c.Password) > 1, len(c.Token) > 1, len(c.PrivateKeyJSON) > 1, len(c.AccessKeyID) > 1 && len(c.SecretAccessKey) > 1),
								Value: sn,
							})
						}
					}
					return res
				}(nc),
			}.scan()
			if nc[n] == nil {
				fmt.Println("Scope name not found")
			} else {
				if strings.HasPrefix(n, "remote-") {
					nc[n].collectRemote()
				} else {
					nc[n].collect()
				}
			}
			continue
		case "delete":
			if len(nc) == 0 {
				fmt.Println("Nothing found, try create new configure")
				continue
			}
			n := scanInput{
				Placeholder: "Which config do you want to edit?\n" + fmt.Sprintf("# %-15s\t%-20s\t%-10s\t%-8s\t%-15s\t%-15s", "ConfigureScope", "Username", "Password", "Token", "PrivateKeyJSON", "AccessKey&Secret"),
				Options: func(conf nsConfigure) []*inputOptions {
					var res []*inputOptions
					for sn, c := range conf {
						if strings.HasPrefix(sn, "remote-") {
							res = append(res, &inputOptions{
								Label: fmt.Sprintf("%-15s\t%-20s\t%-10s\t%-8t\t%-15t\t%-15t\n", sn, c.BucketName, c.Region, len(c.Token) > 1, len(c.PrivateKeyJSON) > 1, len(c.AccessKeyID) > 1 && len(c.SecretAccessKey) > 1),
								Value: sn,
							})
						} else {
							res = append(res, &inputOptions{
								Label: fmt.Sprintf("%-15s\t%-20s\t%-10t\t%-8t\t%-15t\t%-15t", sn, c.Username, len(c.Password) > 1, len(c.Token) > 1, len(c.PrivateKeyJSON) > 1, len(c.AccessKeyID) > 1 && len(c.SecretAccessKey) > 1),
								Value: sn,
							})
						}
					}
					return res
				}(nc),
			}.scan()
			if nc[n] == nil {
				fmt.Println("Scope name not found")
			} else {
				nc[n] = nil
				delete(nc, n)
			}
			continue
		case "print":
			nc.printCurrentConfig()
			continue
		case "quit":
			break
		}
		break
	}
	return nc
}

func download(urls ...string) {

}
func (config *configure) collectRemote() {
	config.Provider = scanInput{
		Default:     "s3",
		Placeholder: "chose cloud storage provider\nNote, only aws s3 supported",
		Minlength:   1,
		Options: []*inputOptions{
			&inputOptions{Value: "s3", Label: "aws s3"},
		},
	}.scan()
	config.Region = scanInput{
		Placeholder: "input bucket region : ",
		Minlength:   3,
		Default:     config.Region,
	}.scan()
	config.BucketName = scanInput{
		Placeholder: "input bucket name : ",
		Minlength:   1,
		Default:     config.BucketName,
	}.scan()
	config.AccessKeyID = scanInput{Placeholder: "Input your access key ID : ", Default: config.AccessKeyID, Minlength: 10}.scan()
	config.SecretAccessKey = scanInput{Placeholder: "Input your secret key : ", Default: config.SecretAccessKey, Password: true, Minlength: 10}.scan()
}
func (config *configure) collect() {
	config.AuthType = scanInput{
		Default:     "basic",
		Placeholder: "Select auth method:",
		Minlength:   1,
		Options: []*inputOptions{
			&inputOptions{Value: "basic", Label: "basic authenticate, vaild username and password are needed"},
			&inputOptions{Value: "token", Label: "token authenticate, vaild token is needed"},
		},
	}.scan()
	for {
		switch config.AuthType {
		case "basic":
			config.Username = scanInput{Placeholder: "Input your username :", Default: config.Username, Minlength: 3}.scan()
			config.Password = scanInput{Placeholder: "Input your password :", Default: config.Password, Password: true, Minlength: 3}.scan()
		case "token":
			config.Token = scanInput{Placeholder: "Input your accessToken, permanent token is recommended :", Default: config.Token, Password: true, Vaild: func(s *string) (bool, error) { return len(*s) == 32, fmt.Errorf("accessToken length must been 32") }}.scan()
		default:
			continue
		}
		break
	}
	/*
		credentialType := scanInput{Placeholder: "Select credential type you prefer:", Options: []*inputOptions{
			&inputOptions{Label: "privateKey, unknow as privateKeyJSON", Value: "privateKey"},
			&inputOptions{Label: "accessID+secretKey, simliar to AWS S3 credentials", Value: "secretKey"},
		}, Default: "privateKey", Minlength: 1}.scan()
		for {
			switch credentialType {
			case "privateKey":
				config.PrivateKeyJSON = scanInput{Placeholder: "Input your private key json file path or base64 encoded string :", Default: config.PrivateKeyJSON, Password: true, Vaild: func(s *string) (bool, error) {
					if len(*s) < 1 {
						return false, fmt.Errorf("cannot been null")
					}
					b, e := ioutil.ReadFile(*s) //os.OpenFile(config.PrivateKeyJSON, os.O_RDONLY, 0600)
					if e != nil {
						_, e := base64.StdEncoding.DecodeString(config.PrivateKeyJSON)
						if e != nil || config.PrivateKeyJSON == "" {
							return false, fmt.Errorf("Input error, no such file and input is not vaild base64 encoded string")
						}
					} else {
						// read config from file
						if !json.Valid(b) {
							return false, fmt.Errorf("The private key json file doesn't vaild JSON content")
						}
						*s = base64.StdEncoding.EncodeToString(b)
					}
					return true, nil
				}}.scan()
			case "secretKey":
				config.AccessKeyID = scanInput{Placeholder: "Input your access key ID : ", Default: config.AccessKeyID, Minlength: 10}.scan()
				config.SecretAccessKey = scanInput{Placeholder: "Input your secret key : ", Default: config.SecretAccessKey, Password: true, Minlength: 10}.scan()
			default:
				continue
			}
			break
		}*/
}

func inSlice(slice []string, ele string) bool {
	for i := 0; i <= len(slice)-1; i++ {
		if slice[0] == ele {
			return true
		}
	}
	return false
}
