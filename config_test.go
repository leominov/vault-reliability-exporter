package main

import "testing"

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
