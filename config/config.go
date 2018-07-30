package config

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
)

// cmdSet represents a set of commands for the `aliases` and `config` keys
// inside of a `netcfg` configuration file.
type cmdSet struct {
	Addr     string      `yaml:"addr"`     // IP address these commands apply to
	Hostname string      `yaml:"hostname"` // hostname these commands apply to
	Vendor   string      `yaml:"vendor"`   // vendor these commands apply to
	OS       string      `yaml:"os"`       // operating system these commands apply to
	Models   []string    `yaml:"models"`   // models these commands apply to
	Version  string      `yaml:"version"`  // software version these commands apply to
	Cmds     interface{} `yaml:"cmds"`     // configuration commands to run
}

// Config represents a `netcfg` configuration file.
type Config struct {
	Hosts   string        `yaml:"hosts"`   // hosts to configure
	User    string        `yaml:"user"`    // user for host login
	Pass    string        `yaml:"pass"`    // password authentication
	Keys    []string      `yaml:"keys"`    // ssh private keys for authentication
	Accept  string        `yaml:"accept"`  // accept connections to these hosts only
	Timeout time.Duration `yaml:"timeout"` // time to wait to establish a connection
	Aliases []*cmdSet     `yaml:"aliases"` // general command set definitions
	Config  []*cmdSet     `yaml:"config"`  // sets of configuration commands

	name     string // name of this config
	data     string // template data for this config
	contents string // contents of the parsed configuration
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
	v := viper.New()
	v.SetConfigType("yaml")

	// create a template from `src`
	tmpl, err := template.New("cfg").Funcs(template.FuncMap{"password": getPass, "prompt": prompt}).Parse(src)
	if err != nil {
		return nil, err
	}

	// read in any template data
	var data interface{}
	if c.data != "" {
		if err := v.ReadConfig(strings.NewReader(c.data)); err != nil {
			return nil, err
		}
		if err := v.Unmarshal(&data); err != nil {
			return nil, err
		}
	}

	// execute the template with the data
	pr, pw := io.Pipe()
	go func(pw *io.PipeWriter, data interface{}) {
		defer pw.Close()
		if err := tmpl.Execute(pw, &data); err != nil {
			pw.CloseWithError(err)
			return
		}
	}(pw, data)

	// read in results of executing the template
	var buf bytes.Buffer
	if err := v.ReadConfig(io.TeeReader(pr, &buf)); err != nil {
		return nil, err
	}

	// copy the results for use in printing
	b, err := ioutil.ReadAll(&buf)
	if err != nil {
		return nil, err
	}
	c.contents = string(b)

	// unmarshal data into Config
	if err := v.Unmarshal(c); err != nil {
		return nil, err
	}
	return c, nil
}

// getPass is a function used in text templates for prompting for a password.
func getPass() (string, error) {
	fmt.Fprint(os.Stderr, "Password: ")
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
		return "", errors.Errorf("expected string, got %T", v)
	}
	return fmt.Sprintf("%q", val), nil
}

// MapCmds prints a map from options to commands.
//
// TODO: setup keys for easy comparison to (*Client).String()
func MapCmds(cfg *Config) (map[string][]string, error) {
	mapCmd := func(set *cmdSet, v interface{}, cmds map[string][]string) error {
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

// Name returns the name of this Config.
func (c *Config) Name() string {
	return c.name
}

// String returns the string representation of this Config.
func (c *Config) String() string {
	return c.contents
}
