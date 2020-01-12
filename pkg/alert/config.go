package alert

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v2"

	"github.com/thanos-io/thanos/pkg/discovery/dns"
	http_util "github.com/thanos-io/thanos/pkg/http"
)

type AlertingConfig struct {
	Alertmanagers []AlertmanagerConfig `yaml:"alertmanagers"`
}

// AlertmanagerConfig represents a client to a cluster of Alertmanager endpoints.
// TODO(simonpasquier): add support for API version (v1 or v2).
type AlertmanagerConfig struct {
	HTTPClientConfig http_util.ClientConfig    `yaml:"http_config"`
	EndpointsConfig  http_util.EndpointsConfig `yaml:",inline"`
	Timeout          model.Duration            `yaml:"timeout"`
}

func DefaultAlertmanagerConfig() AlertmanagerConfig {
	return AlertmanagerConfig{
		EndpointsConfig: http_util.EndpointsConfig{
			Scheme:          "http",
			StaticAddresses: []string{},
			FileSDConfigs:   []http_util.FileSDConfig{},
		},
		Timeout: model.Duration(time.Second * 10),
	}
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *AlertmanagerConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultAlertmanagerConfig()
	type plain AlertmanagerConfig
	return unmarshal((*plain)(c))
}

// LoadAlertingConfig loads a list of AlertmanagerConfig from YAML data.
func LoadAlertingConfig(confYaml []byte) (AlertingConfig, error) {
	var cfg AlertingConfig
	if err := yaml.UnmarshalStrict(confYaml, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// BuildAlertmanagerConfig initializes and returns an Alertmanager client configuration from a static address.
func BuildAlertmanagerConfig(address string, timeout time.Duration) (AlertmanagerConfig, error) {
	parsed, err := url.Parse(address)
	if err != nil {
		return AlertmanagerConfig{}, err
	}

	scheme := parsed.Scheme
	host := parsed.Host
	for _, qType := range []dns.QType{dns.A, dns.SRV, dns.SRVNoA} {
		prefix := string(qType) + "+"
		if strings.HasPrefix(strings.ToLower(scheme), prefix) {
			// Scheme is of the form "<dns type>+<http scheme>".
			scheme = strings.TrimPrefix(scheme, prefix)
			host = prefix + parsed.Host
			if qType == dns.A {
				if _, _, err := net.SplitHostPort(parsed.Host); err != nil {
					// The host port could be missing. Append the defaultAlertmanagerPort.
					host = host + ":" + strconv.Itoa(defaultAlertmanagerPort)
				}
			}
			break
		}
	}
	var basicAuth http_util.BasicAuth
	if parsed.User != nil && parsed.User.String() != "" {
		basicAuth.Username = parsed.User.Username()
		pw, _ := parsed.User.Password()
		basicAuth.Password = pw
	}

	return AlertmanagerConfig{
		HTTPClientConfig: http_util.ClientConfig{
			BasicAuth: basicAuth,
		},
		EndpointsConfig: http_util.EndpointsConfig{
			PathPrefix:      parsed.Path,
			Scheme:          scheme,
			StaticAddresses: []string{host},
		},
		Timeout: model.Duration(timeout),
	}, nil
}