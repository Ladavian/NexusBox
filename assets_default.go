package main

import (
	"embed"
)

//go:embed configdns.yaml
var defaultConfigFS embed.FS

const defaultConfigFile = "configdns.yaml"

// getDefaultConfig 返回内置默认配置的内容
func getDefaultConfig() ([]byte, error) {
	return defaultConfigFS.ReadFile(defaultConfigFile)
}
