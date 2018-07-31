package config

import (
	"testing"
	"time"
	"fmt"
)

const (
	noError  = true
	hasError = false
)

// TODO: Use data from testdata folder.
const (
	options = `
---
hosts: hosts.txt
user: user
pass: password
keys:
  - /home/user/.ssh/id_rsa
accept: all
timeout: 10s
`
	aliases = `
---
aliases:
  - &cisco_default
    vendor: cisco
    cmds: &cisco_cmds
      0: show lldp neighbors
      1: quit

config:
  - *cisco_default
  - <<: *cisco_default
    models:
      - c2960s
    cmds:
      <<: *cisco_cmds
      1: write mem
      2: quit
`
	tmpl = `
---
config:
  {{- range .template}}
  - hostname: {{.hostname}}
    cmds:
      - snmp-agent location {{.location}}
  {{- end}}
`
	tmplData = `
---
template:
  - hostname: host1
    location: host1_snmp_location
  - hostname: host2
    location: host2_snmp_location
`
	tmplContents = `
---
config:
  - hostname: host1
    cmds:
      - snmp-agent location host1_snmp_location
  - hostname: host2
    cmds:
      - snmp-agent location host2_snmp_location
`
	tmplFuncs = `
---
pass: {{password "testing123"}}
config:
  - cmds:
      - {{prompt "N"}}
`
	tmplFuncsContents = `
---
pass: testing123
config:
  - cmds:
      - "N"
`
)

type configTest struct {
	name string
	data string
	src  string
	ok   bool
	want *Config
}

var configParseTests = []configTest{
	{
		name: "empty",
		data: "",
		src:  "",
		ok:   hasError,
		want: nil,
	},
	{
		name: "options",
		data: "",
		src:  options,
		ok:   noError,
		want: &Config{
			Hosts:   "hosts.txt",
			User:    "user",
			Pass:    "password",
			Keys:    []string{"/home/user/.ssh/id_rsa"},
			Accept:  "all",
			Timeout: 10 * time.Second,
		},
	},
	{
		name: "aliases",
		data: "",
		src:  aliases,
		ok:   noError,
		want: &Config{
			Aliases: []cmdSet{
				{Vendor: "cisco", Cmds: map[interface{}]interface{}{0: "show lldp nieghbors", 1: "quit"}},
			},
			Config: []cmdSet{
				{Vendor: "cisco", Cmds: map[interface{}]interface{}{0: "show lldp nieghbors", 1: "quit"}},
				{Vendor: "cisco", Models: []string{"c2960s"},
					Cmds: map[interface{}]interface{}{0: "show lldp nieghbors", 1: "write mem", 2: "quit"},
				},
			},
		},
	},
	{
		name: "template",
		data: tmplData,
		src:  tmpl,
		ok:   noError,
		want: &Config{
			Config: []cmdSet{
				{Hostname: "host1", Cmds: []interface{}{"snmp-agent location host1_snmp_location"}},
				{Hostname: "host2", Cmds: []interface{}{"snmp-agent location host2_snmp_location"}},
			},
		},
	},
	{
		name: "template functions",
		data: "",
		src:  tmplFuncs,
		ok:   noError,
		want: &Config{Pass: "testing123", Config: []cmdSet{{Cmds: []interface{}{"N"}}}},
	},
}

func TestNew(t *testing.T) {
	name := "test"
	got := New(name)
	want := &Config{name: name}
	if !configsEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestConfig_Template(t *testing.T) {
	const tmplSrc = `
---
template:
  cisco:
    - host: cisco_host
      location: cisco_snmp_location
  hp:
    - host: hp_host
      location: hp_snmp_location_2
`
	got := New("tmpl").Template(tmplSrc)
	want := &Config{name: "tmpl", data: tmplSrc}
	if !configsEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestConfig_Name(t *testing.T) {
	want := "test"
	got := New(want)
	if got.Name() != want {
		t.Errorf("want %s, got %s", want, got.Name())
	}
}

func TestConfig_String(t *testing.T) {
	got, err := New("test").Parse(options)
	if err != nil {
		t.Error(err)
	}
	if got.String() != got.contents || got.String() != options {
		t.Errorf("want %s, got %s", options, got.String())
	}
}

func TestConfig_Parse(t *testing.T) {
	for _, test := range configParseTests {
		if test.want != nil {
			test.want.name = test.name
			test.want.data = test.data
			switch test.name {
			case "template":
				test.want.contents = tmplContents
			case "template functions":
				test.want.contents = tmplFuncsContents
			default:
				test.want.contents = test.src
			}
		}
		t.Run(test.name, func(t *testing.T) {
			got, err := New(test.name).Template(test.data).Parse(test.src)
			switch {
			case err != nil && test.ok:
				t.Errorf("unexpected error: %v", err)
			case err == nil && !test.ok:
				t.Error("expected error, got none")
			case err != nil && !test.ok:
				t.Logf("got expected error: %v", err)
			}
			if !configsEqual(got, test.want) {
				t.Errorf("\nwant: %v\ngot: %v", test.want, got)
			}
		})
	}
}

func TestMapCmds(t *testing.T) {
	cfg, err := New("cfg").Parse(aliases)
	if err != nil {
		t.Fatal(err)
	}
	cmds, err := MapCmds(cfg)
	if err != nil {
		t.Error(err)
	}
	s := "IP Addr: %s, Hostname: %q, Vendor: %q, OS: %q, Model: %q, Version: %q"
	ciscoDefaultCmds, ok := cmds[fmt.Sprintf(s, "", "", "cisco", "", "", "")]
	if !ok {
		t.Errorf("key not in cmdMap")
	}
	if !slicesEqual(ciscoDefaultCmds, []string{"show lldp neighbors", "quit"}) {
		t.Errorf("commands do not match")
	}
	ciscoModifiedCmds, ok := cmds[fmt.Sprintf(s, "", "", "cisco", "", "c2960s", "")]
	if !ok {
		t.Errorf("key not in cmdMap")
	}
	if !slicesEqual(ciscoModifiedCmds, []string{"show lldp neighbors", "write mem", "quit"}) {
		t.Errorf("commands do not match")
	}
}

func configsEqual(x, y *Config) bool {
	if x == nil || y == nil {
		return x == nil && y == nil
	}
	return x.name == y.name &&
		x.data == y.data &&
		x.contents == y.contents &&
		optionsEqual(x, y) &&
		cmdSetsEqual(x.Aliases, y.Aliases) &&
		cmdSetsEqual(x.Config, y.Config)
}

func optionsEqual(x, y *Config) bool {
	return x.Hosts == y.Hosts &&
		x.User == y.User &&
		x.Pass == y.Pass &&
		slicesEqual(x.Keys, y.Keys) &&
		x.Accept == y.Accept &&
		x.Timeout == y.Timeout
}

func cmdSetsEqual(x, y []cmdSet) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		xset := x[i]
		yset := y[i]
		if xset.Addr != yset.Addr &&
			xset.Hostname != yset.Hostname &&
			xset.Vendor != yset.Vendor &&
			xset.OS != yset.OS &&
			xset.Version != yset.Version &&
			slicesEqual(xset.Models, yset.Models) &&
			slicesEqual(toStringSlice(xset.Cmds), toStringSlice(xset.Cmds)) {
			return false
		}

	}
	return true
}

func slicesEqual(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func toStringSlice(x interface{}) (s []string) {
	switch v := x.(type) {
	case []interface{}:
		for i := 0; i < len(v); i++ {
			s = append(s, v[i].(string))
		}
	case map[interface{}]interface{}:
		for i := 0; i < len(v); i++ {
			s = append(s, v[i].(string))
		}
	}
	return s
}
