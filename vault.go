package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/vault/api"
)

const (
	userPassAuthMethod = "userpass"
	ldapAuthMethod     = "ldap"
	tokenAuthMethod    = "token"
	appRoleAuthMethod  = "approle"
)

var (
	supportedAuthMethods = map[string]bool{
		userPassAuthMethod: true,
		ldapAuthMethod:     true,
		tokenAuthMethod:    true,
		appRoleAuthMethod:  true,
	}
)

func NewClient(addr, user, pass, token, roleID, secretID, authMethod string, timeout time.Duration) (*api.Client, error) {
	authMethod = strings.ToLower(authMethod)
	_, ok := supportedAuthMethods[authMethod]
	if !ok {
		return nil, fmt.Errorf("Unsupported aith method: %s", authMethod)
	}

	config := api.Config{
		Address: addr,
	}
	client, err := api.NewClient(&config)
	if err != nil {
		return nil, err
	}
	client.SetClientTimeout(timeout)

	options := make(map[string]interface{})
	path := fmt.Sprintf("auth/%s/login", authMethod)
	switch authMethod {
	case tokenAuthMethod:
		client.SetToken(token)
		return client, nil
	case userPassAuthMethod, ldapAuthMethod:
		path = fmt.Sprintf("auth/%s/login/%s", authMethod, user)
		options = map[string]interface{}{
			"password": pass,
		}
	case appRoleAuthMethod:
		options = map[string]interface{}{
			"role_id":   roleID,
			"secret_id": secretID,
		}
	}

	secret, err := client.Logical().Write(path, options)
	if err != nil {
		return nil, err
	}

	client.SetToken(secret.Auth.ClientToken)
	return client, nil
}
