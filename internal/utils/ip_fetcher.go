package utils

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
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

// ValidateIPv4 检查IP地址是否为合法的IPv4格式
func ValidateIPv4(ip string) error {
	// 检查IP合法性（只允许IPv4，禁止IPv6、JSON、报错信息、内容过长等）
	if len(ip) > 40 {
		return fmt.Errorf("IP address too long: %s", ip)
	}

	if strings.Contains(ip, ":") {
		return fmt.Errorf("IPv6 not supported: %s", ip)
	}

	if strings.ContainsAny(ip, "[{") {
		return fmt.Errorf("invalid IP format (contains JSON markers): %s", ip)
	}

	if strings.Contains(strings.ToLower(ip), "error") {
		return fmt.Errorf("error response received: %s", ip)
	}

	if strings.Contains(strings.ToLower(ip), "html") {
		return fmt.Errorf("HTML response received: %s", ip)
	}

	// 严格正则校验IPv4
	ipv4Pattern := `^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`
	matched, err := regexp.MatchString(ipv4Pattern, ip)
	if err != nil {
		return fmt.Errorf("regex validation error: %v", err)
	}

	if !matched {
		return fmt.Errorf("invalid IPv4 format: %s", ip)
	}

	return nil
}

// GetValidatedPublicIP 获取并验证公网IP地址
func GetValidatedPublicIP(configService interface{}) (string, error) {
	var currentIP string
	var err error

	// 尝试从配置服务获取IP查询URL
	if configService != nil {
		// 使用接口断言来访问GetConfig方法
		if cs, ok := configService.(interface{ GetConfig(string) (string, error) }); ok {
			ipFetchURL, configErr := cs.GetConfig("ip_fetch_url")
			if configErr != nil || ipFetchURL == "" {
				ipFetchURL = "https://4.ipw.cn" // 默认URL
			}
			currentIP, err = GetPublicIPWithURL(ipFetchURL)
		} else {
			// 降级到默认方法
			currentIP, err = GetPublicIP()
		}
	} else {
		// 降级到默认方法
		currentIP, err = GetPublicIP()
	}

	if err != nil {
		return "", fmt.Errorf("failed to get public IP: %v", err)
	}

	// 验证IP格式
	if err := ValidateIPv4(currentIP); err != nil {
		return "", fmt.Errorf("IP validation failed: %v", err)
	}

	// logger.Printf("Successfully got and validated public IP: %s", currentIP)
	return currentIP, nil
}
