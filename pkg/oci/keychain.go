package oci

import (
	"strings"

	ecrlogin "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/google/go-containerregistry/pkg/authn"
)

// ECRKeychain returns a keychain that tries the ECR credential helper for ECR
// registries and falls back to the default keychain for everything else (e.g.
// ghcr.io, docker.io). The ECR helper returns an error for non-ECR hosts, so
// we wrap it to return authn.Anonymous instead, allowing MultiKeychain to
// continue to the next entry.
func ECRKeychain() authn.Keychain {
	return authn.NewMultiKeychain(
		&ecrFallbackKeychain{helper: ecrlogin.NewECRHelper()},
		authn.DefaultKeychain,
	)
}

type ecrFallbackKeychain struct {
	helper *ecrlogin.ECRHelper
}

func (e *ecrFallbackKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	registry := target.RegistryStr()
	if !isECRRegistry(registry) {
		return authn.Anonymous, nil
	}
	username, password, err := e.helper.Get(registry)
	if err != nil {
		return authn.Anonymous, nil
	}
	return authn.FromConfig(authn.AuthConfig{
		Username: username,
		Password: password,
	}), nil
}

func isECRRegistry(registry string) bool {
	return strings.Contains(registry, ".dkr.ecr.") && strings.Contains(registry, ".amazonaws.com")
}
