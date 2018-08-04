package device

import (
	"fmt"
	"testing"
)

func TestClient_Addr(t *testing.T) {
	addr := "127.0.0.1"
	c := &Client{addr: addr}
	if c.addr != addr || c.Addr() != addr || c.Addr() != c.addr {
		t.Errorf("want %s, got %s", addr, c.addr)
	}
}

func TestClient_Hostname(t *testing.T) {
	hostname := "localhost"
	c := &Client{hostname: hostname}
	if c.hostname != hostname || c.Hostname() != hostname || c.Hostname() != c.hostname {
		t.Errorf("want %s, got %s", hostname, c.hostname)
	}
}

func TestClient_Vendor(t *testing.T) {
	vendor := "cisco"
	c := &Client{vendor: vendor}
	if c.vendor != vendor || c.Vendor() != vendor || c.Vendor() != c.vendor {
		t.Errorf("want %s, got %s", vendor, c.vendor)
	}
}

func TestClient_OS(t *testing.T) {
	os := "IOS"
	c := &Client{os: os}
	if c.os != os || c.OS() != os || c.OS() != c.os {
		t.Errorf("want %s, got %s", os, c.addr)
	}
}

func TestClient_Model(t *testing.T) {
	model := "c2960"
	c := &Client{model: model}
	if c.model != model || c.Model() != model || c.Model() != c.model {
		t.Errorf("want %s, got %s", model, c.model)
	}
}

func TestClient_Version(t *testing.T) {
	version := "15.0(2)SE10a"
	c := &Client{version: version}
	if c.version != version || c.Version() != version || c.Version() != c.version {
		t.Errorf("want %s, got %s", version, c.version)
	}
}

func TestClient_String(t *testing.T) {
	want := fmt.Sprintf("IP Addr: %s, Hostname: %s, Vendor: %s, OS: %s, Model: %s, Version: %s",
		"127.0.0.1", "localhost", "cisco", "ios", "c2960s", "15.0(2)SE10a")
	c := &Client{nil, "127.0.0.1", "localhost", "cisco", "ios", "c2960s", "15.0(2)SE10a"}
	if c.String() != want {
		t.Errorf("want %s, got %s", want, c.String())
	}
}

func TestGetSysDescr(t *testing.T) {
	want := map[string]string{
		"addr":     "",
		"hostname": "",
		"vendor":   "",
		"os":       "",
		"model":    "",
		"version":  "",
	}
	info := make(chan map[string]string)
	go getSysDescr("127.0.0.1", info)
	got, ok := <-info
	if !ok {
		t.Fatal("channel closed")
	}
	if len(got) != len(want) {
		t.Fatal("maps not same length")
	}
	for k, v := range want {
		val, ok := got[k]
		if !ok {
			t.Fatalf("key %s not in map", k)
		}
		if val != v {
			t.Errorf("want %s, got %s", v, val)
		}
	}
}

var testDescrs = []struct {
	name  string
	descr string
	want  map[string]string
}{
	{"empty", "", map[string]string{
		"addr":     "",
		"hostname": "",
		"vendor":   "",
		"os":       "",
		"model":    "",
		"version":  "",
	}},
	{
		name:  "unknown",
		descr: "NetApp Release RironcityN_080806_2230: Wed Aug 6 23:55:19 PDT 2008",
		want: map[string]string{
			"addr":     "",
			"hostname": "",
			"vendor":   "",
			"os":       "",
			"model":    "",
			"version":  "",
		},
	},
	{
		name: "cisco IOS s72033_rp",
		descr: `Cisco IOS Software, s72033_rp Software (s72033_rp-ADVENTERPRISEK9_WAN-M), Version 12.2(33)SXJ10, RELEASE SOFTWARE (fc3)
Technical Support: http://www.cisco.com/techsupport
Copyright (c) 1986-2015 by Cisco Systems, Inc.
Compiled Fri 14-Aug-15 08:58 by p`,
		want: map[string]string{
			"addr":     "",
			"hostname": "",
			"vendor":   "CISCO",
			"os":       "IOS",
			"model":    "s72033_rp",
			"version":  "s72033_rp-ADVENTERPRISEK9_WAN-M Version 12.2(33)SXJ10 RELEASE SOFTWARE (fc3)",
		},
	},
	{
		name: "cisco IOS c2960s",
		descr: `Cisco IOS Software, C2960S Software (C2960S-UNIVERSALK9-M), Version 15.0(2)SE10a, RELEASE SOFTWARE (fc3)
Technical Support: http://www.cisco.com/techsupport
Copyright (c) 1986-2016 by Cisco Systems, Inc.
Compiled Thu 03-Nov-16 13:52`,
		want: map[string]string{
			"addr":     "",
			"hostname": "",
			"vendor":   "CISCO",
			"os":       "IOS",
			"model":    "C2960S",
			"version":  "C2960S-UNIVERSALK9-M Version 15.0(2)SE10a RELEASE SOFTWARE (fc3)",
		},
	},
	{
		name: "Cisco IOS XE cat3k_caa",
		descr: `Cisco IOS Software, IOS-XE Software, Catalyst L3 Switch Software (CAT3K_CAA-UNIVERSALK9-M), Version 03.06.06.E RELEASE SOFTWARE (fc1)
Technical Support: http://www.cisco.com/techsupport
Copyright (c) 1986-2016 by Cisco Systems, Inc.
Compiled Sat 17-Dec`,
		want: map[string]string{
			"addr":     "",
			"hostname": "",
			"vendor":   "CISCO",
			"os":       "IOS XE",
			"model":    "CAT3K_CAA",
			"version":  "CAT3K_CAA-UNIVERSALK9-M Version 03.06.06.E RELEASE SOFTWARE (fc1)",
		},
	},
	{
		name: "Cisco IOS XE cat4500e",
		descr: `Cisco IOS Software, IOS-XE Software, Catalyst 4500 L3 Switch Software (cat4500e-UNIVERSALK9-M), Version 03.04.00.SG RELEASE SOFTWARE (fc3)
Technical Support: http://www.cisco.com/techsupport
Copyright (c) 1986-2012 by Cisco Systems, Inc.
Compiled Wed 0`,
		want: map[string]string{
			"addr":     "",
			"hostname": "",
			"vendor":   "CISCO",
			"os":       "IOS XE",
			"model":    "cat4500e",
			"version":  "cat4500e-UNIVERSALK9-M Version 03.04.00.SG RELEASE SOFTWARE (fc3)",
		},
	},
	{
		name:  "Cisco NX-OS",
		descr: `Cisco NX-OS(tm) n6000, Software (n6000-uk9), Version 7.1(1)N1(1), RELEASE SOFTWARE Copyright (c) 2002-2012 by Cisco Systems, Inc. Device Manager Version 6.0(2)N1(1),Compiled 4/18/2015 10:00:00`,
		want: map[string]string{
			"addr":     "",
			"hostname": "",
			"vendor":   "CISCO",
			"os":       "NX-OS",
			"model":    "n6000",
			"version":  "n6000-uk9 Version 7.1(1)N1(1)",
		},
	},
	{
		name: "Cisco IOS XR",
		descr: `Cisco IOS XR Software (Cisco ASR9K Series),  Version 5.3.4[Default]
Copyright (c) 2016 by Cisco Systems, Inc.`,
		want: map[string]string{
			"addr":     "",
			"hostname": "",
			"vendor":   "CISCO",
			"os":       "IOS XR",
			"model":    "ASR9K",
			"version":  "Version 5.3.4[Default]",
		},
	},
	{
		name: "HP Comware",
		descr: `HPE Comware Platform Software, Software Version 7.1.070, Release 1309
HPE 5130 48G PoE+ 4SFP+ 1-slot HI Switch JH326A
Copyright (c) 2010-2017 Hewlett Packard Enterprise Development LP`,
		want: map[string]string{
			"addr":     "",
			"hostname": "",
			"vendor":   "HP",
			"os":       "Comware",
			"model":    "HPE 5130 48G PoE+ 4SFP+ 1-slot HI Switch JH326A",
			"version":  "Software Version 7.1.070, Release 1309",
		},
	},
	{
		name:  "HP ProCurve",
		descr: `ProCurve J9145A 2910al-24G Switch, revision W.14.03, ROM W.14.04 (/sw/code/build/sbm(t4a_RC3))`,
		want: map[string]string{
			"addr":     "",
			"hostname": "",
			"vendor":   "HP",
			"os":       "ProCurve",
			"model":    "ProCurve J9145A 2910al-24G Switch,",
			"version":  "revision W.14.03, ROM W.14.04",
		},
	},
}

func TestParseSysDescr(t *testing.T) {
	for _, test := range testDescrs {
		t.Run(test.name, func(t *testing.T) {
			got := parseSysDescr(test.descr)
			if len(got) != len(test.want) {
				t.Fatal("maps do not have same length")
			}
			for k, v := range test.want {
				val, ok := got[k]
				if !ok {
					t.Fatalf("key %s not in map", k)
				}
				if val != v {
					t.Errorf("want %s, got %s", v, val)
				}
			}
		})
	}
}
