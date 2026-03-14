package keystore

import (
	"github.com/zalando/go-keyring"
)

const (
	serviceName = "com.alexkovalov.geminichat"
	apiKeyUser  = "gemini_api_key"
)

func SaveAPIKey(key string) error {
	return keyring.Set(serviceName, apiKeyUser, key)
}

func GetAPIKey() (string, error) {
	return keyring.Get(serviceName, apiKeyUser)
}

func DeleteAPIKey() error {
	return keyring.Delete(serviceName, apiKeyUser)
}
