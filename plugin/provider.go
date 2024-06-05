
package plugin

import (
    "errors"
)

type BackendProvider interface {
    SetupBackend(map[string]string) error
    GetDockerfileAdditions() string
}

var providers = make(map[string]BackendProvider)

func RegisterProvider(providerType string, provider BackendProvider) {
    providers[providerType] = provider
}

func GetProvider(providerType string) (BackendProvider, error) {
    provider, exists := providers[providerType]
    if !exists {
        return nil, errors.New("unknown provider type")
    }
    return provider, nil
}


