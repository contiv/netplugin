package utils

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	logrus_syslog "github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/contiv/netplugin/core"
	"github.com/urfave/cli"
	"log/syslog"
	"net/url"
	"strings"
	"time"
)

// BuildNetworkFlags CLI networking flags for given binary
func BuildNetworkFlags(binary string) []cli.Flag {
	binUpper := strings.ToUpper(binary)
	binLower := strings.ToLower(binary)
	return []cli.Flag{
		cli.StringFlag{
			Name:   "mode, plugin-mode, cluster-mode",
			EnvVar: fmt.Sprintf("CONTIV_%s_MODE", binUpper),
			Usage:  fmt.Sprintf("set %s mode, options: [docker, kubernetes, swarm-mode]", binLower),
		},
		cli.StringFlag{
			Name:   "netmode, network-mode",
			EnvVar: fmt.Sprintf("CONTIV_%s_NET_MODE", binUpper),
			Usage:  fmt.Sprintf("set %s network mode, options: [vlan, vxlan]", binLower),
		},
		cli.StringFlag{
			Name:   "fwdmode, forward-mode",
			EnvVar: fmt.Sprintf("CONTIV_%s_FORWARD_MODE", binUpper),
			Usage:  fmt.Sprintf("set %s forwarding network mode, options: [bridge, routing]", binLower),
		},
		/*
			// only ovs is supported
			// TODO: turn it on when having more than one backend supported
			cli.StringFlag {
				Name: "driver, net-driver",
				Value: "ovs",
				EnvVar: "CONTIV_NETPLUGIN_DRIVER",
				Usage: "set netplugin key-value store url, options: [ovs, vpp]",
			}
		*/
	}
}

// NetworkConfigs validated net configs
type NetworkConfigs struct {
	Mode        string
	NetworkMode string
	ForwardMode string
}

// BuildDBFlags CLI storage flags for given binary
func BuildDBFlags(binary string) []cli.Flag {
	binUpper := strings.ToUpper(binary)
	binLower := strings.ToLower(binary)
	return []cli.Flag{
		cli.StringFlag{
			Name:   "etcd-endpoints, etcd",
			EnvVar: fmt.Sprintf("CONTIV_%s_ETCD_ENDPOINTS", binUpper),
			Usage:  fmt.Sprintf("a comma-delimited list of %s etcd endpoints (default: http://127.0.0.1:2379)", binLower),
		},
		cli.StringFlag{
			Name:   "consul-endpoints, consul",
			EnvVar: fmt.Sprintf("CONTIV_%s_CONSUL_ENDPOINTS", binUpper),
			Usage:  fmt.Sprintf("a comma-delimited list of %s consul endpoints", binLower),
		},
	}
}

// DBConfigs validated db configs
type DBConfigs struct {
	StoreDriver string
	StoreURL    string
}

// BuildLogFlags CLI logging flags for given binary
func BuildLogFlags(binary string) []cli.Flag {
	binUpper := strings.ToUpper(binary)
	binLower := strings.ToLower(binary)
	return []cli.Flag{
		cli.StringFlag{
			Name:   "log-level",
			Value:  "INFO",
			EnvVar: fmt.Sprintf("CONTIV_%s_LOG_LEVEL", binUpper),
			Usage:  fmt.Sprintf("set %s log level, options: [DEBUG, INFO, WARN, ERROR]", binLower),
		},
		cli.BoolFlag{
			Name:   "use-json-log, json-log",
			EnvVar: fmt.Sprintf("CONTIV_%s_USE_JSON_LOG", binUpper),
			Usage:  fmt.Sprintf("set %s log format to json if this flag is provided", binLower),
		},
		cli.BoolFlag{
			Name:   "use-syslog, syslog",
			EnvVar: fmt.Sprintf("CONTIV_%s_USE_SYSLOG", binUpper),
			Usage:  fmt.Sprintf("set %s send log to syslog if this flag is provided", binLower),
		},
		cli.StringFlag{
			Name:   "syslog-url",
			Value:  "udp://127.0.0.1:514",
			EnvVar: fmt.Sprintf("CONTIV_%s_SYSLOG_URL", binUpper),
			Usage:  fmt.Sprintf("set %s syslog url in format protocol://ip:port", binLower),
		},
	}
}

func configureSyslog(binary string, loglevel logrus.Level, syslogRawURL string) error {
	var err error
	var hook logrus.Hook
	var syslogURL *url.URL
	var priority syslog.Priority

	// disable colors if we're writing to syslog *and* we're the default text
	// formatter, because the tty detection is useless here.
	if tf, ok := logrus.StandardLogger().Formatter.(*logrus.TextFormatter); ok {
		tf.DisableColors = true
	}

	syslogURL, err = url.Parse(syslogRawURL)
	if err != nil {
		return fmt.Errorf("Failed parsing syslog spec %q: %v", syslogRawURL, err.Error())
	}

	switch loglevel {
	case logrus.PanicLevel, logrus.FatalLevel:
		priority = syslog.LOG_CRIT
	case logrus.ErrorLevel:
		priority = syslog.LOG_ERR
	case logrus.WarnLevel:
		priority = syslog.LOG_WARNING
	case logrus.InfoLevel:
		priority = syslog.LOG_INFO
	case logrus.DebugLevel:
		priority = syslog.LOG_DEBUG
	}

	hook, err = logrus_syslog.NewSyslogHook(syslogURL.Scheme, syslogURL.Host, priority, binary)
	if err != nil {
		return fmt.Errorf("Failed connecting to syslog %q: %v", syslogRawURL, err.Error())
	}

	logrus.AddHook(hook)
	return nil
}

// InitLogging initiates logging from CLI options
func InitLogging(binary string, ctx *cli.Context) error {
	logLevel, err := logrus.ParseLevel(ctx.String("log-level"))
	if err != nil {
		return err
	}
	logrus.SetLevel(logLevel)
	logrus.Infof("Using %v log level: %v", binary, logLevel)

	if ctx.Bool("use-syslog") {
		syslogURL := ctx.String("syslog-url")
		if err := configureSyslog(binary, logLevel, syslogURL); err != nil {
			return err
		}
		logrus.Infof("Using %v syslog config: %v", binary, syslogURL)
	} else {
		logrus.Infof("Using %v syslog config: nil", binary)
	}

	if ctx.Bool("use-json-log") {
		logrus.SetFormatter(&logrus.JSONFormatter{})
		logrus.Infof("Using %v log format: json", binary)
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true, TimestampFormat: time.StampNano})
		logrus.Infof("Using %v log format: text", binary)
	}
	return nil
}

// ValidateDBOptions returns error if db options are not valid
func ValidateDBOptions(binary string, ctx *cli.Context) (*DBConfigs, error) {
	var storeDriver string
	var storeURL string
	var storeURLs string
	etcdURLs := ctx.String("etcd")
	consulURLs := ctx.String("consul")

	if etcdURLs != "" && consulURLs != "" {
		return nil, fmt.Errorf("ambiguous %s db endpoints, both etcd and consul specified: etcd: %s, consul: %s", binary, etcdURLs, consulURLs)
	} else if etcdURLs == "" && consulURLs == "" {
		// if neither etcd or consul is set, try etcd at http://127.0.0.1:2379
		storeDriver = "etcd"
		storeURLs = "http://127.0.0.1:2379"
	} else if etcdURLs != "" {
		storeDriver = "etcd"
		storeURLs = etcdURLs
	} else {
		storeDriver = "consul"
		storeURLs = consulURLs
	}
	for _, endpoint := range FilterEmpty(strings.Split(storeURLs, ",")) {
		_, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid %s %v endpoint: %v", binary, storeDriver, endpoint)
		}
		// TODO: support multi-endpoints
		storeURL = endpoint
		logrus.Infof("Using %s state db endpoints: %v: %v", binary, storeDriver, storeURL)
		break
	}

	if storeURL == "" {
		return nil, fmt.Errorf("invalid %s %s endpoints: empty", binary, storeDriver)
	}

	return &DBConfigs{
		StoreDriver: storeDriver,
		StoreURL:    storeURL,
	}, nil
}

// ValidateNetworkOptions returns error if network options are not valid
func ValidateNetworkOptions(binary string, ctx *cli.Context) (*NetworkConfigs, error) {
	// 1. validate and set plugin mode
	pluginMode := strings.ToLower(ctx.String("mode"))
	switch pluginMode {
	case core.Docker, core.Kubernetes, core.SwarmMode, core.Test:
		logrus.Infof("Using %s mode: %v", binary, pluginMode)
	case "":
		return nil, fmt.Errorf("%s mode is not set", binary)
	default:
		return nil, fmt.Errorf("unknown %s mode: %v", binary, pluginMode)
	}

	// 2. validate and set network mode
	networkMode := strings.ToLower(ctx.String("netmode"))
	switch networkMode {
	case "vlan", "vxlan":
		logrus.Infof("Using %s network mode: %v", binary, networkMode)
	case "":
		return nil, fmt.Errorf("%s network mode is not set", binary)
	default:
		return nil, fmt.Errorf("unknown %s network mode: %v", binary, networkMode)
	}

	// 3. validate forwarding mode
	forwardMode := strings.ToLower(ctx.String("fwdmode"))
	if forwardMode == "" {
		return nil, fmt.Errorf("unknown %s forwarding mode: %v", binary, forwardMode)
	} else if forwardMode != "bridge" && forwardMode != "routing" {
		return nil, fmt.Errorf("%s forwarding mode is not set", binary)
	} else if networkMode == "vxlan" && forwardMode == "bridge" {
		return nil, fmt.Errorf("invalid %s forwarding mode: %q (network mode: %q)", binary, forwardMode, networkMode)
	}
	// vxlan/vlan+routing, vlan+bridge are valid combinations
	logrus.Infof("Using %s forwarding mode: %v", binary, forwardMode)
	return &NetworkConfigs{
		Mode:        pluginMode,
		NetworkMode: networkMode,
		ForwardMode: forwardMode,
	}, nil
}

// FlattenFlags concatenate slices of flags into one slice
func FlattenFlags(flagSlices ...[]cli.Flag) []cli.Flag {
	var flags []cli.Flag
	for _, slice := range flagSlices {
		flags = append(flags, slice...)
	}
	return flags
}

// FilterEmpty filters empty string from string slices
func FilterEmpty(stringSlice []string) []string {
	var result []string
	for _, str := range stringSlice {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}
