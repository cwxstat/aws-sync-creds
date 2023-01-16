package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/spf13/viper"
	"log"
	"os"
	"strings"
	"time"
)

type Cache struct {
	ProviderType string `json:"ProviderType"`
	Credentials  struct {
		AccessKeyID     string    `json:"AccessKeyId"`
		SecretAccessKey string    `json:"SecretAccessKey"`
		SessionToken    string    `json:"SessionToken"`
		Expiration      time.Time `json:"Expiration"`
	} `json:"Credentials"`
}

type DB struct {
	C       *Cache
	FileDir string
}

type Identity struct {
	STS     *sts.GetCallerIdentityOutput
	Profile string
	Cache   *Cache
}

type DBs struct {
	m map[string]Identity
}

func NewDBs() *DBs {
	d := &DBs{}
	d.m = make(map[string]Identity, 0)
	return d
}

func (d *DBs) AddCache(c *Cache, fileDir string) {

	cfg, err := Config(*c)
	if err != nil {
		return
	}

	result, err := sts.NewFromConfig(cfg).GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return
	}
	d.m[fileDir] = Identity{
		STS:     result,
		Profile: profile(*result.Account, *result.Arn),
		Cache:   c,
	}
}

func profile(account, arn string) string {
	s := strings.Split(arn, "_")
	if len(s) >= 2 {
		return account + "_" + s[1]
	}
	return ""
}

func (d *DBs) Key(fileDir string) bool {
	if _, ok := d.m[fileDir]; ok {
		return true
	}
	return false
}

func (c *Cache) IsExpired() bool {
	return c.Credentials.Expiration.Before(time.Now())
}

func BuildDBs(dir string) (*DBs, error) {
	d := NewDBs()
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for k, v := range files {
		if strings.HasSuffix(v.Name(), "json") {
			c, err := ReadFile(dir + v.Name())
			if err != nil {
				continue
			}
			if !c.IsExpired() && !d.Key(dir+v.Name()) {
				fmt.Println(k, c)
				d.AddCache(c, dir+v.Name())
			}

		}
	}
	return d, nil
}

func (d *DBs) List() *NP {
	np := NewNP()
	for _, v := range d.m {
		np.Add(v.Profile+".aws_access_key_id", v.Cache.Credentials.AccessKeyID)
		np.Add(v.Profile+".aws_secret_access_key", v.Cache.Credentials.SecretAccessKey)
		np.Add(v.Profile+".aws_session_token", v.Cache.Credentials.SessionToken)
	}
	return np
}

func ReadFile(file string) (*Cache, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var c Cache
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func Config(c Cache) (aws.Config, error) {
	err := os.Setenv("AWS_ACCESS_KEY_ID", c.Credentials.AccessKeyID)
	if err != nil {
		return aws.Config{}, err
	}
	os.Setenv("AWS_SECRET_ACCESS_KEY", c.Credentials.SecretAccessKey)
	if err != nil {
		return aws.Config{}, err
	}
	os.Setenv("AWS_SESSION_TOKEN", c.Credentials.SessionToken)
	if err != nil {
		return aws.Config{}, err
	}

	os.Unsetenv("AWS_PROFILE")
	os.Unsetenv("AWS_SDK_LOAD_CONFIG")
	os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")
	os.Unsetenv("AWS_CONFIG_FILE")
	os.Unsetenv("AWS_CA_BUNDLE")
	os.Unsetenv("AWS_DEFAULT_REGION")
	os.Unsetenv("AWS_REGION")

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	return cfg, nil
}

func AllProfiles() ([]string, error) {
	viper.AddConfigPath("$HOME/.aws")
	viper.SetConfigName("credentials") // Register config file name (no extension)
	viper.SetConfigType("ini")         // Look for specific type
	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	return viper.AllKeys(), nil
}

type NewProfile struct {
	Key   string
	Value string
}

type NP struct {
	Profiles []NewProfile
}

func NewNP() *NP {
	return &NP{}
}
func (n *NP) Add(key, value string) {
	n.Profiles = append(n.Profiles, NewProfile{
		Key:   key,
		Value: value,
	})
}

func (n *NP) List() []NewProfile {
	return n.Profiles
}

// SetProfile sets the profile in the credentials file
//
//	 path := "$HOME/.aws"
//		name := "credentials"
func SetProfile(path, name string, np *NP) error {

	viper.AddConfigPath(path)
	viper.SetConfigName(name)  // Register config file name (no extension)
	viper.SetConfigType("ini") // Look for specific type
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	// Viper removes default profile
	r := viper.Get("default")

	for _, v := range np.List() {
		viper.Set(v.Key, v.Value)
	}

	err = viper.WriteConfig()

	// Hack Viper removes default profile
	if r != nil {
		if strings.Contains(path, "$HOME") {
			path = strings.Replace(path, "$HOME", os.Getenv("HOME"), -1)
		}
		dat, err := os.ReadFile(path + "/" + name)
		if err != nil {
			return err
		}
		if !strings.Contains(string(dat), "[default]") {
			return os.WriteFile(path+"/"+name, []byte("[default]\n"+string(dat)), 0600)
		}
	}
	return err
}

func Sync() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	d, err := BuildDBs(home + "/.aws/cli/cache/")
	if err != nil {
		return err
	}
	fmt.Println(d)

	path := "$HOME/.aws"
	name := "credentials"
	return SetProfile(path, name, d.List())

}