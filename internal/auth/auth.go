package auth

import (
	"encoding/base64"
	"errors"

	"engineering-flow-platform-tools/internal/config"
)

func AuthHeaders(a config.AuthConfig) (map[string]string, error) {
	a.NormalizeType()
	switch a.Type {
	case "basic_password":
		if a.Username == "" || a.Password == "" {
			return nil, errors.New("config_error")
		}
		return map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(a.Username+":"+a.Password))}, nil
	case "basic_api_key":
		secret := a.APIKey
		if secret == "" {
			secret = a.Token
		}
		if a.Username == "" || secret == "" {
			return nil, errors.New("config_error")
		}
		return map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(a.Username+":"+secret))}, nil
	case "bearer_token":
		if a.Token == "" {
			return nil, errors.New("config_error")
		}
		return map[string]string{"Authorization": "Bearer " + a.Token}, nil
	default:
		return nil, errors.New("config_error")
	}
}
