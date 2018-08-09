package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
	data []byte // template data for this config
	text string // text of the parsed configuration
}

// New creates a new configuration.
func New(name string) *Config {
	return &Config{name: name}
}

// Data adds template data to a Config for use cfg parsing.
func (c *Config) Data(src []byte) *Config {
	if c == nil {
		return nil
	}
	c.data = src
	return c
}

// Parse parses a Config with any template data.
func (c *Config) Parse(src string) (*Config, error) {
	if c == nil {
		return nil, errors.New("cannot parse a nil Config")
	}
	if src == "" {
		return nil, errors.New("nothing to parse, config file is empty")
	}

	// create a new text template
	tmpl, err := template.New("cfg").Funcs(template.FuncMap{"password": getPass, "prompt": prompt}).Parse(src)
	if err != nil {
		return nil, fmt.Errorf("could not parse template %s: %v", tmpl.Name(), err)
	}

	// create a new viper instance
	v := viper.New()
	v.SetConfigType("yaml")

	// read cfg any config data
	if err := v.ReadConfig(bytes.NewReader(c.data)); err != nil {
		return nil, fmt.Errorf("could not read config data: %v", err)
	}

	// unmarshal any config data
	var data interface{}
	if err := v.Unmarshal(&data); err != nil {
		return nil, fmt.Errorf("could not unmarshal config data: %v", err)
	}

	// execute the text template and copy into a pipe
	pr, pw := io.Pipe()
	go func() {
		if err := tmpl.Execute(pw, &data); err != nil {
			pw.CloseWithError(fmt.Errorf("could not execute %s: %v", tmpl.Name(), err))
			return
		}
		pw.Close()
	}()

	// read the executed text template's contents and
	// copy the output to a buffer
	var buf bytes.Buffer
	if err := v.ReadConfig(io.TeeReader(pr, &buf)); err != nil {
		return nil, fmt.Errorf("could not read config: %v", err)
	}
	c.text = buf.String()

	// unmarshal the text template's contents into the Config
	if err := v.Unmarshal(c); err != nil {
		return nil, fmt.Errorf("could not unmarshal config: %v", err)
	}
	return c, nil
}

// getPass is a function used cfg text templates for prompting for a password.
func getPass(s ...string) (string, error) {
	var (
		in  *os.File
		out io.Writer
	)
	if len(s) != 0 && len(s) != 2 {
		return "", fmt.Errorf("expected 0 or 2 args, got %d", len(s))
	}
	if len(s) == 2 && s[0] == "test" {
		f, err := ioutil.TempFile("", "")
		if err != nil {
			return "", nil
		}
		defer func() {
			f.Close()
			os.Remove(f.Name())
		}()
		in = f
		out = ioutil.Discard
		if _, err := fmt.Fprintf(out, "Password: "); err != nil {
			return "", err
		}
		if _, err := io.WriteString(in, s[1]); err != nil {
			return "", err
		}
		pass, err := ioutil.ReadFile(in.Name())
		if err != nil {
			return "", nil
		}
		return string(pass), nil
	}
	in = os.Stdin
	out = os.Stdout
	if _, err := fmt.Fprintf(out, "Password: "); err != nil {
		return "", err
	}
	pass, err := terminal.ReadPassword(int(in.Fd()))
	if err != nil {
		return "", fmt.Errorf("could not read password: %v", err)
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return "", err
	}
	return string(pass), nil
}

// prompt is a function used cfg text templates to enter the specified value
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
func (c *Config) Cmds() (map[string][]string, error) {
	if c == nil {
		return nil, errors.New("could not map commands: config is nil")
	}
	cmds := make(map[string][]string, len(c.Config))
	for _, set := range c.Config {
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
			return nil, fmt.Errorf("could not map commands: expected sequence or map, got %T", v)
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
