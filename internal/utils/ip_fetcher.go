package utils

import (
	"io"
	"net/http"
	"strings"
)

// GetPublicIP fetches the public IP from an external service.
func GetPublicIP() (string, error) {
	resp, err := http.Get("https://4.ipw.cn")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(ip)), nil
}
