//go:build !linux

package main

import "errors"

var errNoService = errors.New("Benutzerdienst nur unter Linux (systemd --user)")

func unitDir() (string, error) { return "", errNoService }

var systemctl = func(args ...string) error { return errNoService }
