package main

import (
	"fmt"
	"os"
)

func getCfgVar(envvar string) (string, error) {
	envval, found := os.LookupEnv(envvar)
	if !found {
		return "", fmt.Errorf("env variable %s must be set", envvar)
	}
	return envval, nil
}
