package main

import (
	"fmt"
	"time"

	"github.com/hashicorp/vault/api"
)

func NewClient(addr, user, pass, authMethod string, timeout time.Duration) (*api.Client, error) {
	config := api.Config{
		Address: addr,
	}
	client, err := api.NewClient(&config)
	if err != nil {
		return nil, err
	}
	client.SetClientTimeout(timeout)
	options := map[string]interface{}{
		"password": pass,
	}
	path := fmt.Sprintf("auth/%s/login/%s", authMethod, user)
	secret, err := client.Logical().Write(path, options)
	if err != nil {
		return nil, err
	}
	client.SetToken(secret.Auth.ClientToken)
	return client, nil
}
