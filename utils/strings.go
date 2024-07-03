package utils

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
	"math/rand"
	"strings"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func ListOutput(rels []string, output string) []string {
	switch strings.ToLower(output) {
	case "yaml":
		d, _ := yaml.Marshal(rels)
		return []string{string(d)}
	case "json":
		d, _ := json.Marshal(rels)
		return []string{string(d)}
	default:
		return rels
	}
}
