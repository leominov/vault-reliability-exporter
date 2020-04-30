package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

var (
	envPrefix = "VRE"

	defaultRepeatInterval = time.Second

	defaultPGWAddr      = "127.0.0.1:9091"
	defaultPGWNamespace = "vault_reliability_exporter"
	defaultPGWJob       = "vault_reliability_job"
	defaultPGWTimeout   = 30 * time.Second

	defaultVaultAddr       = "https://127.0.0.1:8200"
	defaultVaultTimeout    = 30 * time.Second
	defaultVaultMaxRetries = 2
	defaultSecretData      = map[string]interface{}{
		"foo": "bar",
	}
	defaultVaultProfile = &VaultProfile{
		AuthPath: "auth/userpass/login/guest",
		AuthData: map[string]interface{}{
			"password": "guest",
		},
	}

	defaultKubernetesJWTTokenLocation = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

type Config struct {
	PGW            PushgatewayOptions `yaml:"pgw_config"`
	Vault          VaultOptions       `yaml:"vault_config"`
	RepeatInterval time.Duration      `yaml:"repeat_interval"`
	Delay          time.Duration      `yaml:"delay"`
	Telemetry      TelemetryOptions   `yaml:"telemetry_config"`
}

type TelemetryOptions struct {
	PushgatewayEnabled *bool `yaml:"pushgateway_enabled"`
	HTTPEnabled        *bool `yaml:"http_enabled"`
}

type PushgatewayOptions struct {
	Addr      string            `yaml:"url"`
	Timeout   time.Duration     `yaml:"timeout"`
	BasicAuth *BasicAuth        `yaml:"basic_auth"`
	Namespace string            `yaml:"namespace"`
	Instance  string            `yaml:"instance"`
	Job       string            `yaml:"job"`
	Labels    map[string]string `yaml:"labels"`
}

type BasicAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type VaultOptions struct {
	Addr       string          `yaml:"url"`
	Timeout    time.Duration   `yaml:"timeout"`
	MaxRetries int             `yaml:"max_retries"`
	Profiles   []*VaultProfile `yaml:"profiles"`
}

type VaultProfile struct {
	Name        string                 `yaml:"name"`
	AuthPath    string                 `yaml:"auth_path"`
	AuthData    map[string]interface{} `yaml:"auth_data,omitempty"`
	AuthToken   string                 `yaml:"auth_token,omitempty"`
	RevokeToken bool                   `yaml:"revoke_token"`
	SecretPath  string                 `yaml:"secret_path"`
	SecretData  map[string]interface{} `yaml:"secret_data"`
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
	return c.ProcessVaultProfiles()
}

func (c *Config) ProcessVaultProfiles() error {
	for _, profile := range c.Vault.Profiles {
		err := profile.ProcessAuthData()
		if err != nil {
			return err
		}
	}
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
	if p.Timeout <= 0 {
		p.Timeout = defaultPGWTimeout
	}
}

func (v *VaultOptions) SetDefaults() {
	if len(v.Addr) == 0 {
		v.Addr = defaultVaultAddr
	}
	if v.Timeout <= 0 {
		v.Timeout = defaultVaultTimeout
	}
	if v.MaxRetries < 0 {
		v.MaxRetries = defaultVaultMaxRetries
	}
	if len(v.Profiles) == 0 {
		v.Profiles = append(v.Profiles, defaultVaultProfile)
	}
	for i, profile := range v.Profiles {
		if len(profile.Name) == 0 {
			profile.Name = fmt.Sprintf("profile%d", i)
		}
		profile.LoadFromEnv()
		profile.SetDefaults()
	}
}

func (v *VaultProfile) authDataEnvKey() string {
	return fmt.Sprintf("%s_PROFILE_%s_AUTH_DATA", envPrefix, strings.ToUpper(v.Name))
}

func (v *VaultProfile) authTokenEnvKey() string {
	return fmt.Sprintf("%s_PROFILE_%s_AUTH_TOKEN", envPrefix, strings.ToUpper(v.Name))
}

func (v *VaultProfile) authPathEnvKey() string {
	return fmt.Sprintf("%s_PROFILE_%s_AUTH_PATH", envPrefix, strings.ToUpper(v.Name))
}

func (v *VaultProfile) LoadFromEnv() {
	if authDataRaw, ok := os.LookupEnv(v.authDataEnvKey()); ok {
		var authData map[string]interface{}
		if err := json.Unmarshal([]byte(authDataRaw), &authData); err == nil {
			v.AuthData = authData
		}
	}
	if authToken, ok := os.LookupEnv(v.authTokenEnvKey()); ok {
		v.AuthToken = authToken
	}
	if authPath, ok := os.LookupEnv(v.authPathEnvKey()); ok {
		v.AuthPath = authPath
	}
}

func (v *VaultProfile) SetDefaults() {
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

func (t *TelemetryOptions) SetDefaults() {
	if t.PushgatewayEnabled == nil {
		t.PushgatewayEnabled = Bool(true)
	}
	if t.HTTPEnabled == nil {
		t.HTTPEnabled = Bool(false)
	}
}

func IsJWTShortcut(key string, val interface{}) bool {
	if strings.ToLower(key) != "jwt" {
		return false
	}
	str, ok := val.(string)
	if !ok {
		return false
	}
	return strings.ToLower(str) == "%jwt%"
}

func (v *VaultProfile) processKubernetesJWTToken(key string, val interface{}) error {
	if !IsJWTShortcut(key, val) {
		return nil
	}
	b, err := ioutil.ReadFile(defaultKubernetesJWTTokenLocation)
	if err != nil {
		return err
	}
	v.AuthData[key] = string(b)
	return nil
}

func (v *VaultProfile) ProcessAuthData() error {
	for key, val := range v.AuthData {
		err := v.processKubernetesJWTToken(key, val)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) SetDefaults() {
	if c.RepeatInterval <= 0 {
		c.RepeatInterval = defaultRepeatInterval
	}
	if c.Delay < 0 {
		c.Delay = 0
	}
	c.PGW.SetDefaults()
	c.Vault.SetDefaults()
	c.Telemetry.SetDefaults()
}

func (c *Config) String() string {
	b, _ := yaml.Marshal(c)
	return string(b)
}
