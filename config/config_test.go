package config

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	name := "test"
	got := New(name)
	want := &Config{name: name}
	if !configsEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestConfig_Name(t *testing.T) {
	var cfg *Config
	if cfg.Name() != "<nil>" {
		t.Errorf("want <nil>, got %v", cfg.Name())
	}
	name := "test"
	cfg = New(name)
	if cfg.Name() != name {
		t.Errorf("want %s, got %s", name, cfg.name)
	}
}

func TestConfig_Data(t *testing.T) {
	data, err := ioutil.ReadFile(filepath.Join("testdata", "tmpl_data.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg *Config
	if cfg.Data(data) != nil {
		t.Errorf("expected nil, got %v", cfg)
	}
	got := New("cfg").Data(data)
	want := &Config{name: "cfg", data: data}
	if got == nil {
		t.Errorf("config is nil, expected %v", want)
	}
	if !bytes.Equal(got.data, data) {
		t.Errorf("want %v, got %v", data, got.data)
	}
	if !configsEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

type configTest struct {
	name string
	cfg  *Config
	src  string
	data string
	want *Config
	ok   bool
}

var configTests = []configTest{
	{"nil", nil, "empty.yml", "", nil, false},
	{"empty", New("empty"), "empty.yml", "", nil, false},
	{
		name: "data",
		cfg:  New("data"),
		src:  "tmpl.yml",
		data: "tmpl_data.yml",
		want: &Config{
			Config: []cmdSet{
				{Hostname: "host1", Cmds: []interface{}{"snmp-agent location host1_snmp_location"}},
				{Hostname: "host2", Cmds: []interface{}{"snmp-agent location host2_snmp_location"}},
			},
			name: "data",
			text: `---
config:
  - hostname: host1
    cmds:
      - snmp-agent location host1_snmp_location
  - hostname: host2
    cmds:
      - snmp-agent location host2_snmp_location`,
		},
		ok: true,
	},
	{
		name: "funcs",
		cfg:  New("funcs"),
		src:  "tmpl_funcs.yml",
		data: "",
		want: &Config{
			Pass: "testing123",
			Config: []cmdSet{
				{Cmds: []interface{}{`"N"`}},
			},
			name: "funcs",
			text: `---
pass: testing123
config:
- cmds:
  - "N"`,
		},
		ok: true,
	},
}

func TestConfig_Parse(t *testing.T) {
	for _, test := range configTests {
		t.Run(test.name, func(t *testing.T) {
			src, err := ioutil.ReadFile(filepath.Join("testdata", test.src))
			if err != nil {
				t.Fatal(err)
			}
			data := make([]byte, 0)
			if test.data != "" {
				b, err := ioutil.ReadFile(filepath.Join("testdata", test.data))
				if err != nil {
					t.Fatal(err)
				}
				data = b
				test.want.data = b
			}
			got, err := test.cfg.Data(data).Parse(string(src))
			switch {
			case err != nil && test.ok:
				t.Errorf("unexpected error: %v", err)
			case err == nil && !test.ok:
				t.Errorf("expected error, got none")
			case err != nil && !test.ok:
				t.Logf("got expected error: %v", err)
			}
			if !configsEqual(got, test.want) {
				t.Errorf("want %v, got %v", test.want, got)
			}
		})
	}
}

func TestConfig_String(t *testing.T) {
	var cfg *Config
	if cfg.String() != "<nil>" {
		t.Errorf("want <nil>, got %s", cfg.String())
	}
	src, err := ioutil.ReadFile(filepath.Join("testdata", "options.yml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err = New("test").Parse(string(src))
	if err != nil {
		t.Fatal(err)
	}
	want := `---
hosts: hosts.txt
user: user
pass: password
keys:
  - /home/user/.ssh/id_rsa
accept: all
timeout: 10s`
	if cfg.String() != want {
		t.Errorf("want %q, got %q", want, cfg.String())
	}
}

func configsEqual(x, y *Config) bool {
	if x == nil || y == nil {
		return x == nil && y == nil
	}
	return x.name == y.name &&
		bytes.Equal(x.data, y.data) &&
		x.text == y.text &&
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
	default:
		return make([]string, 0)
	}
	return s
}
