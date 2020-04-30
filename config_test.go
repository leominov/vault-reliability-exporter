package main

import (
	"os"
	"testing"
)

func TestIsJWTShortcut(t *testing.T) {
	tests := []struct {
		key string
		val interface{}
		out bool
	}{
		{
			key: "foo",
			val: "bar",
			out: false,
		},
		{
			key: "jwt",
			val: nil,
			out: false,
		},
		{
			key: "jwt",
			val: []string{
				"%jwt%",
			},
			out: false,
		},
		{
			key: "jwt",
			val: "%jwt%",
			out: true,
		},
	}
	for id, test := range tests {
		res := IsJWTShortcut(test.key, test.val)
		if res != test.out {
			t.Errorf("%d. Must be %v, but got %v", id, test.out, res)
		}
	}
}

func TestVaultProfile_LoadFromEnv(t *testing.T) {
	p := &VaultProfile{
		Name: "profile1",
	}

	os.Unsetenv(p.authDataEnvKey())
	os.Unsetenv(p.authTokenEnvKey())
	os.Unsetenv(p.authPathEnvKey())

	p.LoadFromEnv()
	if p.AuthData != nil {
		t.Errorf("Must be nil, but got %v", p.AuthData)
	}
	if p.AuthToken != "" {
		t.Errorf("Must be empty, but got %s", p.AuthToken)
	}
	if p.AuthPath != "" {
		t.Errorf("Must be empty, but got %s", p.AuthPath)
	}

	os.Setenv(p.authDataEnvKey(), `{"foo": "bar"}`)
	os.Setenv(p.authTokenEnvKey(), `foobar`)
	os.Setenv(p.authPathEnvKey(), "foo/bar")

	p.LoadFromEnv()
	if p.AuthToken == "" {
		t.Error("Must be non-empty")
	}
	if p.AuthData == nil {
		t.Fatal("Must be non-nil")
	}
	val, ok := p.AuthData["foo"]
	if !ok {
		t.Fatalf("Can't find %q field", "foo")
	}
	if val.(string) != "bar" {
		t.Errorf("Must be %q, but got %v", "bar", p.AuthData["foo"])
	}
	if p.AuthPath == "" {
		t.Error("Must be non-empty")
	}

	p = &VaultProfile{
		Name: "profile1",
	}

	os.Setenv(p.authDataEnvKey(), `{"foo": "ba`)

	p.LoadFromEnv()

	if p.AuthData != nil {
		t.Errorf("Must be nil, but got %v", p.AuthData)
	}
}
