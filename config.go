package main

import (
	"fmt"
	"io/ioutil"
	"time"

	yaml "gopkg.in/yaml.v2"
)

var (
	defaultRepeatInterval = time.Second

	defaultPGWAddr      = "127.0.0.1:9091"
	defaultPGWNamespace = "vault_reliability_exporter"
	defaultPGWJob       = "vault_reliability_job"

	defaultVaultAddr    = "https://127.0.0.1:8200"
	defaultVaultTimeout = 30 * time.Second
	defaultSecretPath   = "probe-secrets/test"
	defaultSecretData   = map[string]interface{}{
		"foo": "bar",
	}
	defaultVaultProfile = &VaultProfile{
		AuthPath: "auth/userpass/login/guest",
		AuthData: map[string]interface{}{
			"password": "guest",
		},
	}
)

type Config struct {
	PGW            PushgatewayOptions `yaml:"pgw_config"`
	Vault          VaultOptions       `yaml:"vault_config"`
	RepeatInterval time.Duration      `yaml:"repeat_interval"`
}

type PushgatewayOptions struct {
	Addr      string            `yaml:"url"`
	Namespace string            `yaml:"namespace"`
	Job       string            `yaml:"job"`
	Labels    map[string]string `yaml:"labels"`
}

type VaultOptions struct {
	Addr     string          `yaml:"url"`
	Timeout  time.Duration   `yaml:"timeout"`
	Profiles []*VaultProfile `yaml:"profiles"`
}

type VaultProfile struct {
	Name       string                 `yaml:"name"`
	AuthPath   string                 `yaml:"auth_path"`
	AuthData   map[string]interface{} `yaml:"auth_data,omitempty"`
	AuthToken  string                 `yaml:"auth_token,omitempty"`
	SecretPath string                 `yaml:"secret_path"`
	SecretData map[string]interface{} `yaml:"secret_data"`
}

func (c *Config) LoadFromFile(filename string) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return err
	}
	c.SetDefaults()
	return nil
}

func (p *PushgatewayOptions) SetDefaults() {
	if len(p.Addr) == 0 {
		p.Addr = defaultPGWAddr
	}
	if len(p.Namespace) == 0 {
		p.Namespace = defaultPGWNamespace
	}
	if len(p.Job) == 0 {
		p.Job = defaultPGWJob
	}
}

func (v *VaultOptions) SetDefaults() {
	if len(v.Addr) == 0 {
		v.Addr = defaultVaultAddr
	}
	if v.Timeout <= 0 {
		v.Timeout = defaultVaultTimeout
	}
	if len(v.Profiles) == 0 {
		v.Profiles = append(v.Profiles, defaultVaultProfile)
	}
	for i, profile := range v.Profiles {
		if len(profile.Name) == 0 {
			profile.Name = fmt.Sprintf("profile%d", i)
		}
		profile.SetDefaults()
	}
}

func (v *VaultProfile) SetDefaults() {
	if len(v.SecretPath) == 0 {
		v.SecretPath = defaultSecretPath
	}
	if len(v.SecretData) == 0 || v.SecretData == nil {
		v.SecretData = defaultSecretData
	}
	if len(v.AuthToken) > 0 {
		v.AuthData = nil
	}
	if len(v.AuthData) > 0 {
		v.AuthToken = ""
	}
}

func (c *Config) SetDefaults() {
	if c.RepeatInterval <= 0 {
		c.RepeatInterval = defaultRepeatInterval
	}
	c.PGW.SetDefaults()
	c.Vault.SetDefaults()
}

func (c *Config) String() string {
	b, _ := yaml.Marshal(c)
	return string(b)
}
