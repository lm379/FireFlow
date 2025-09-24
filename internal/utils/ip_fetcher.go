package utils

import (
	"io"
	"net/http"
	"strings"
)

// GetPublicIP fetches the public IP from an external service.
func GetPublicIP() (string, error) {
	return GetPublicIPWithURL("https://4.ipw.cn")
}

// GetPublicIPWithURL fetches the public IP from a specified URL.
func GetPublicIPWithURL(url string) (string, error) {
	if url == "" {
		url = "https://4.ipw.cn" // 默认URL
	}

	resp, err := http.Get(url)
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
