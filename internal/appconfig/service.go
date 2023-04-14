package appconfig

import (
	"fmt"

	"github.com/samber/lo"
	"github.com/superfly/flyctl/api"
	"github.com/superfly/flyctl/internal/sentry"
)

type Service struct {
	Protocol     string                         `json:"protocol,omitempty" toml:"protocol"`
	InternalPort int                            `json:"internal_port,omitempty" toml:"internal_port"`
	Ports        []api.MachinePort              `json:"ports,omitempty" toml:"ports"`
	Concurrency  *api.MachineServiceConcurrency `json:"concurrency,omitempty" toml:"concurrency"`
	TCPChecks    []*ServiceTCPCheck             `json:"tcp_checks,omitempty" toml:"tcp_checks,omitempty"`
	HTTPChecks   []*ServiceHTTPCheck            `json:"http_checks,omitempty" toml:"http_checks,omitempty"`
	Processes    []string                       `json:"processes,omitempty" toml:"processes,omitempty"`
}

type ServiceTCPCheck struct {
	Interval    *api.Duration `json:"interval,omitempty" toml:"interval,omitempty"`
	Timeout     *api.Duration `json:"timeout,omitempty" toml:"timeout,omitempty"`
	GracePeriod *api.Duration `toml:"grace_period,omitempty" json:"grace_period,omitempty"`
	// RestartLimit is only supported on V1 Apps
	RestartLimit int `toml:"restart_limit,omitempty" json:"restart_limit,omitempty"`
}

type ServiceHTTPCheck struct {
	Interval    *api.Duration `json:"interval,omitempty" toml:"interval,omitempty"`
	Timeout     *api.Duration `json:"timeout,omitempty" toml:"timeout,omitempty"`
	GracePeriod *api.Duration `toml:"grace_period,omitempty" json:"grace_period,omitempty"`
	// RestartLimit is only supported on V1 Apps
	RestartLimit int `toml:"restart_limit,omitempty" json:"restart_limit,omitempty"`

	// HTTP Specifics
	HTTPMethod        *string           `json:"method,omitempty" toml:"method,omitempty"`
	HTTPPath          *string           `json:"path,omitempty" toml:"path,omitempty"`
	HTTPProtocol      *string           `json:"protocol,omitempty" toml:"protocol,omitempty"`
	HTTPTLSSkipVerify *bool             `json:"tls_skip_verify,omitempty" toml:"tls_skip_verify,omitempty"`
	HTTPHeaders       map[string]string `json:"headers,omitempty" toml:"headers,omitempty"`
}

type HTTPService struct {
	InternalPort int                            `json:"internal_port,omitempty" toml:"internal_port" validate:"required,numeric"`
	ForceHTTPS   bool                           `toml:"force_https" json:"force_https,omitempty"`
	Concurrency  *api.MachineServiceConcurrency `toml:"concurrency,omitempty" json:"concurrency,omitempty"`
	Processes    []string                       `json:"processes,omitempty" toml:"processes,omitempty"`
}

func (s *HTTPService) ToService() *Service {
	return &Service{
		Protocol:     "tcp",
		InternalPort: s.InternalPort,
		Concurrency:  s.Concurrency,
		Processes:    s.Processes,
		Ports: []api.MachinePort{{
			Port:       api.IntPointer(80),
			Handlers:   []string{"http"},
			ForceHttps: s.ForceHTTPS,
		}, {
			Port:     api.IntPointer(443),
			Handlers: []string{"http", "tls"},
		}},
		TCPChecks:  nil,
		HTTPChecks: nil,
	}
}

func (c *Config) AllServices() (services []Service) {
	if c.HTTPService != nil {
		services = append(services, *c.HTTPService.ToService())
	}
	services = append(services, c.Services...)
	return services
}

func (svc *Service) toMachineService() *api.MachineService {
	s := &api.MachineService{
		Protocol:     svc.Protocol,
		InternalPort: svc.InternalPort,
		Ports:        svc.Ports,
		Concurrency:  svc.Concurrency,
	}

	for _, tc := range svc.TCPChecks {
		s.Checks = append(s.Checks, *tc.toMachineCheck())
	}
	for _, hc := range svc.HTTPChecks {
		s.Checks = append(s.Checks, *hc.toMachineCheck())
	}
	return s
}

func (chk *ServiceHTTPCheck) toMachineCheck() *api.MachineCheck {
	return &api.MachineCheck{
		Type:              api.Pointer("http"),
		Interval:          chk.Interval,
		Timeout:           chk.Timeout,
		GracePeriod:       chk.GracePeriod,
		HTTPMethod:        chk.HTTPMethod,
		HTTPPath:          chk.HTTPPath,
		HTTPProtocol:      chk.HTTPProtocol,
		HTTPSkipTLSVerify: chk.HTTPTLSSkipVerify,
		HTTPHeaders: lo.MapToSlice(
			chk.HTTPHeaders, func(k string, v string) api.MachineHTTPHeader {
				return api.MachineHTTPHeader{Name: k, Values: []string{v}}
			}),
	}
}

func (chk *ServiceHTTPCheck) String(port int) string {
	return fmt.Sprintf("http-%d-%v", port, chk.HTTPMethod)
}

func (chk *ServiceTCPCheck) toMachineCheck() *api.MachineCheck {
	return &api.MachineCheck{
		Type:        api.Pointer("tcp"),
		Interval:    chk.Interval,
		Timeout:     chk.Timeout,
		GracePeriod: chk.GracePeriod,
	}
}

func (chk *ServiceTCPCheck) String(port int) string {
	return fmt.Sprintf("tcp-%d", port)
}

func serviceFromMachineService(ms api.MachineService, processes []string) *Service {
	var (
		tcpChecks  []*ServiceTCPCheck
		httpChecks []*ServiceHTTPCheck
	)
	for _, check := range ms.Checks {
		switch *check.Type {
		case "tcp":
			tcpChecks = append(tcpChecks, tcpCheckFromMachineCheck(check))
		case "http":
			httpChecks = append(httpChecks, httpCheckFromMachineCheck(check))
		default:
			sentry.CaptureException(fmt.Errorf("unknown check type '%s' when converting from machine service", *check.Type))
		}
	}
	return &Service{
		Protocol:     ms.Protocol,
		InternalPort: ms.InternalPort,
		Ports:        ms.Ports,
		Concurrency:  ms.Concurrency,
		TCPChecks:    tcpChecks,
		HTTPChecks:   httpChecks,
		Processes:    processes,
	}
}

func tcpCheckFromMachineCheck(mc api.MachineCheck) *ServiceTCPCheck {
	return &ServiceTCPCheck{
		Interval:     mc.Interval,
		Timeout:      mc.Timeout,
		GracePeriod:  nil,
		RestartLimit: 0,
	}
}

func httpCheckFromMachineCheck(mc api.MachineCheck) *ServiceHTTPCheck {
	headers := make(map[string]string)
	for _, h := range mc.HTTPHeaders {
		if len(h.Values) > 0 {
			headers[h.Name] = h.Values[0]
		}
		if len(h.Values) > 1 {
			sentry.CaptureException(fmt.Errorf("bug: more than one header value provided by MachineCheck, but can only support one value for fly.toml"))
		}
	}
	return &ServiceHTTPCheck{
		Interval:          mc.Interval,
		Timeout:           mc.Timeout,
		GracePeriod:       nil,
		RestartLimit:      0,
		HTTPMethod:        mc.HTTPMethod,
		HTTPPath:          mc.HTTPPath,
		HTTPProtocol:      mc.HTTPProtocol,
		HTTPTLSSkipVerify: mc.HTTPSkipTLSVerify,
		HTTPHeaders:       headers,
	}
}
