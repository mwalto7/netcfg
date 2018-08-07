package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
)

// cmdSet is the set of configurations commands to be run.
type cmdSet struct {
	Addr     string      `yaml:"addr"`     // commands apply to this IP address
	Hostname string      `yaml:"hostname"` // commands apply to this hostname
	Vendor   string      `yaml:"vendor"`   // commands apply to this vendor
	OS       string      `yaml:"os"`       // commands apply to this operating system
	Models   []string    `yaml:"models"`   // commands apply to these models
	Version  string      `yaml:"version"`  // commands apply to this software version
	Cmds     interface{} `yaml:"cmds"`     // configuration commands to run
}

// Config represents a `netcfg` configuration file.
type Config struct {
	Hosts   string        `yaml:"hosts"`   // file of hosts to configure
	User    string        `yaml:"user"`    // username for host login
	Pass    string        `yaml:"pass"`    // password for host login
	Keys    []string      `yaml:"keys"`    // ssh private keys for authentication
	Accept  string        `yaml:"accept"`  // group of hosts to accept connections to
	Timeout time.Duration `yaml:"timeout"` // time to wait to establish an ssh client connection
	Aliases []cmdSet      `yaml:"aliases"` // aliases for configuration command sets
	Config  []cmdSet      `yaml:"config"`  // sets of configuration commands to run

	name string // name of this config
	data string // template data for this config
	text string // text of the parsed configuration
}

// New creates a new configuration.
func New(name string) *Config {
	return &Config{name: name}
}

// Template adds template data to a Config for use in parsing.
func (c *Config) Template(src string) *Config {
	c.data = src
	return c
}

// Parse parses a Config with any template data.
func (c *Config) Parse(src string) (*Config, error) {
	if src == "" {
		return nil, errors.New("nothing to parse, config file is empty")
	}
	tmpl, err := template.New("cfg").Funcs(template.FuncMap{"password": getPass, "prompt": prompt}).Parse(src)
	if err != nil {
		return nil, fmt.Errorf("could not parse template %s: %v", tmpl.Name(), err)
	}

	v := viper.New()
	v.SetConfigType("yaml")

	data, err := unmarshal(v, c.data)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go func(pw *io.PipeWriter, data interface{}) {
		if err := tmpl.Execute(pw, &data); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}(pw, data)

	var buf bytes.Buffer
	if err := v.ReadConfig(io.TeeReader(pr, &buf)); err != nil {
		return nil, err
	}
	c.text = buf.String()

	if err := v.Unmarshal(c); err != nil {
		return nil, err
	}
	return c, nil
}

// unmarshal reads in a src string and decodes the data.
func unmarshal(v *viper.Viper, src string) (data interface{}, err error) {
	if src == "" {
		return
	}
	if err = v.ReadConfig(strings.NewReader(src)); err != nil {
		return
	}
	if err = v.Unmarshal(&data); err != nil {
		return
	}
	return
}

// getPass is a function used in text templates for prompting for a password.
func getPass(s ...string) (string, error) {
	if len(s) > 0 {
		return s[0], nil
	}
	fmt.Fprintf(os.Stderr, "Password: ")
	password, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr)
	return string(password), nil
}

// prompt is a function fused in text templates to enter the specified value
// at an expected prompt on the remote session.
func prompt(v interface{}) (string, error) {
	val, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("expected string, got %T", v)
	}
	return fmt.Sprintf("%q", val), nil
}

// Name returns the name of this Config.
func (c *Config) Name() string {
	if c == nil {
		return "<nil>"
	}
	return c.name
}

// String returns the string representation of this Config.
func (c *Config) String() string {
	if c == nil {
		return "<nil>"
	}
	return c.text
}

// MapCmds prints a map from options to commands.
func MapCmds(cfg *Config) (map[string][]string, error) {
	cmds := make(map[string][]string, len(cfg.Config))
	for _, set := range cfg.Config {
		switch v := set.Cmds.(type) {
		case []interface{}:
			for i := 0; i < len(v); i++ {
				if err := mapCmd(set, v[i], cmds); err != nil {
					return nil, err
				}
			}
		case map[interface{}]interface{}:
			for i := 0; i < len(v); i++ {
				if err := mapCmd(set, v[i], cmds); err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("expected sequence or map, got %T", v)
		}
	}
	return cmds, nil
}

// mapCmd maps a command to its options.
func mapCmd(set cmdSet, v interface{}, cmds map[string][]string) error {
	var keys []string
	s := "IP Addr: %s, Hostname: %q, Vendor: %q, OS: %q, Model: %q, Version: %q"
	if len(set.Models) > 0 {
		for _, model := range set.Models {
			keys = append(keys, fmt.Sprintf(s, set.Addr, set.Hostname, set.Vendor, set.OS, model, set.Version))
		}
	} else {
		keys = append(keys, fmt.Sprintf(s, set.Addr, set.Hostname, set.Vendor, set.OS, "", set.Version))
	}
	cmd, ok := v.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", v)
	}
	for _, k := range keys {
		if k == "" || k == fmt.Sprintf(s, "", "", "", "", "", "") {
			k = "generic"
		} else {
			k = strings.TrimSpace(k)
		}
		cmds[k] = append(cmds[k], cmd)
	}
	return nil
}
