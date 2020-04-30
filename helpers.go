package main

func Bool(v bool) *bool {
	p := new(bool)
	*p = v
	return p
}
