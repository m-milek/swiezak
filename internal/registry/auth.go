package registry

import (
	"log/slog"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
)

type credentials struct {
	username string
	password string
}

type envKeychain map[string]credentials

func (k envKeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	creds, ok := k[resource.RegistryStr()]
	if !ok {
		return authn.Anonymous, nil
	}
	return &authn.Basic{Username: creds.username, Password: creds.password}, nil
}

func KeychainFromEnv() authn.Keychain {
	registries := []struct {
		host   string
		prefix string
	}{
		{host: "index.docker.io", prefix: "DOCKERHUB"},
		{host: "ghcr.io", prefix: "GHCR"},
	}

	keychain := envKeychain{}
	for _, r := range registries {
		username := os.Getenv(r.prefix + "_USERNAME")
		password := os.Getenv(r.prefix + "_TOKEN")
		if username == "" || password == "" {
			if username != "" || password != "" {
				slog.Warn("incomplete registry credentials, using anonymous access", "registry", r.host, "need", r.prefix+"_USERNAME and "+r.prefix+"_TOKEN")
			}
			continue
		}
		keychain[r.host] = credentials{username: username, password: password}
	}
	return keychain
}
