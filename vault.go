package main

import (
	"time"

	"github.com/hashicorp/vault/api"
)

func NewClient(addr string, timeout time.Duration, retries int, profile *VaultProfile) (*api.Client, error) {
	config := api.Config{
		Address:    addr,
		Timeout:    timeout,
		MaxRetries: retries,
	}
	client, err := api.NewClient(&config)
	if err != nil {
		return nil, err
	}
	if len(profile.AuthToken) > 0 {
		client.SetToken(profile.AuthToken)
		return client, nil
	}
	secret, err := client.Logical().Write(profile.AuthPath, profile.AuthData)
	if err != nil {
		return nil, err
	}
	client.SetToken(secret.Auth.ClientToken)
	return client, nil
}
