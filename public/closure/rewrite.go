package closure

import (
	"net/url"
)

// RewriteServers replaces the URL in each server entry with rewriteURL,
// preserving template variables (e.g., {region}) if they appear in the path.
func RewriteServers(servers []interface{}, rewriteURL string) []interface{} {
	if rewriteURL == "" {
		return servers
	}
	target, err := url.Parse(rewriteURL)
	if err != nil {
		return servers
	}
	for _, s := range servers {
		m, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		rawURL, ok := m["url"].(string)
		if !ok {
			continue
		}
		// Parse the original to preserve any path suffix
		orig, err := url.Parse(rawURL)
		if err != nil {
			m["url"] = rewriteURL
			continue
		}
		// Replace scheme + host, keep path if target has no path
		result := *target
		if result.Path == "" || result.Path == "/" {
			result.Path = orig.Path
		}
		m["url"] = result.String()
	}
	return servers
}
