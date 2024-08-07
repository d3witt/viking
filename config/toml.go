package config

import (
	"fmt"
	"net"
)

type IPList []net.IP

func (m *IPList) UnmarshalTOML(data interface{}) error {
	switch v := data.(type) {
	case string:
		ip := net.ParseIP(v)
		if ip == nil {
			return fmt.Errorf("invalid IP address: %s", v)
		}
		*m = []net.IP{ip}
	case []interface{}:
		var result []net.IP
		for _, item := range v {
			if str, ok := item.(string); ok {
				ip := net.ParseIP(str)
				if ip == nil {
					return fmt.Errorf("invalid IP address: %s", str)
				}
				result = append(result, ip)
			} else {
				return fmt.Errorf("expected string in array but got %T", item)
			}
		}
		*m = result
	default:
		return fmt.Errorf("expected string or array of strings but got %T", v)
	}
	return nil
}
