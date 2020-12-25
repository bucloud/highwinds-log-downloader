package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gopkg.in/ini.v1"
)

type configure struct {
	AuthType        string `ini:"auth_type"`
	Username        string `ini:"user_name"`
	Password        string `ini:"password"`
	Token           string `ini:"token"`
	PrivateKeyJSON  string `ini:"private_key_json"`
	AccessKeyID     string `ini:"access_key_id"`
	SecretAccessKey string `ini:"secret_access_key"`
}

type nsConfigure map[string]*configure

var (
	// Cfg global variable contains configure data
	defaultConfigScope string = "DEFAULT"
	configFile         string = homeDir(".highwinds", "hcs.ini")
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

func (nc nsConfigure) printCurrentConfig(scopeName ...string) {
	fmt.Printf("# %-15s\t%-20s\t%-10s\t%-8s\t%-15s\t%-15s", "ConfigureScope", "Username", "Password", "Token", "PrivateKeyJSON", "AccessKey&Secret\n")
	var i int = 1
	if len(scopeName) == 0 {
		for s, c := range nc {
			fmt.Printf("%d %-15s\t%-20s\t%-10t\t%-8t\t%-15t\t%-15t\n", i, s, c.Username, len(c.Password) > 1, len(c.Token) > 1, len(c.PrivateKeyJSON) > 1, len(c.AccessKeyID) > 1 && len(c.SecretAccessKey) > 1)
			i++
		}
	} else {
		for _, c := range scopeName {
			fmt.Printf("%d %-15s\t%-20s\t%-10t\t%-8t\t%-15t\t%-15t\n", i, c, nc[c].Username, len(nc[c].Password) > 1, len(nc[c].Token) > 1, len(nc[c].PrivateKeyJSON) > 1, len(nc[c].AccessKeyID) > 1 && len(nc[c].SecretAccessKey) > 1)
			i++
		}
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
					&inputOptions{Value: "remote", Label: "set config for different account"},
				},
			}.scan()
			switch configScope {
			case "remote":

			case "custom":
				configScope = scanInput{
					Placeholder: "Input accountHash (in order to use configure more effectively, please use accountHash as scope name): ",
					Vaild:       func(s *string) (bool, error) { return len(*s) == 8, fmt.Errorf("sames not a vaild accountHash") },
				}.scan()
			case "default":
				configScope = defaultConfigScope
			default:
				configScope = defaultConfigScope
			}
			if nc[defaultConfigScope] != nil {
				nc[configScope] = nc[defaultConfigScope]
			} else {
				nc[configScope] = &configure{}
			}
			nc[configScope].collect()
			continue
		case "edit":
			if len(nc) == 0 {
				fmt.Println("Nothing found, try create new configure")
				continue
			}
			nc.printCurrentConfig()
			n := scanInput{Placeholder: "Which config do you want to edit? "}.scan()
			if nc[n] == nil {
				fmt.Println("Scope name not found")
			} else {
				nc[n].collect()
			}
			continue
		case "delete":
			if len(nc) == 0 {
				fmt.Println("Nothing found, try create new configure")
				continue
			}
			nc.printCurrentConfig()
			n := scanInput{Placeholder: "Which config do you want to delete? "}.scan()
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
	}
}

func inSlice(slice []string, ele string) bool {
	for i := 0; i <= len(slice)-1; i++ {
		if slice[0] == ele {
			return true
		}
	}
	return false
}
