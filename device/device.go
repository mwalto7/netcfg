package device

import (
	"bytes"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	snmp "github.com/mwalto7/gosnmp"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
)

// Timeout is the duration to wait for an SSH connection to establish.
var Timeout = time.Duration(0)

// Client represents an SSH client for a network device.
type Client struct {
	client   *ssh.Client // underlying SSH client connection
	addr     string      // IP address of the device
	hostname string      // hostname of the device
	vendor   string      // vendor of the device
	os       string      // operating system of the device
	model    string      // model of the device
	version  string      // software version of the device
}

// Dial establishes an SSH client connection to a remote host.
func Dial(host, port string, clientCfg *ssh.ClientConfig) (*Client, error) {
	client, err := ssh.Dial("tcp", net.JoinHostPort(host, port), clientCfg)
	if err != nil {
		return nil, err
	}
	s := strings.Split(client.RemoteAddr().String(), ":")
	addr := strings.Join(s[:len(s)-1], "")
	m := sysDescr(addr)
	c := &Client{
		client:   client,
		addr:     m["addr"],
		hostname: m["hostname"],
		vendor:   m["vendor"],
		os:       m["os"],
		model:    m["model"],
		version:  m["version"],
	}
	return c, nil
}

// Addr returns the remote host's IP address.
func (c *Client) Addr() string {
	if c == nil {
		return "<nil>"
	}
	return c.addr
}

// Hostname returns the remote host's hostname.
func (c *Client) Hostname() string {
	if c == nil {
		return "<nil>"
	}
	return c.hostname
}

// Vendor returns the remote host's vendor.
func (c *Client) Vendor() string {
	if c == nil {
		return "<nil>"
	}
	return c.vendor
}

// OS returns the remote host's operating system.
func (c *Client) OS() string {
	if c == nil {
		return "<nil>"
	}
	return c.os
}

// Model returns the remote host's model.
func (c *Client) Model() string {
	if c == nil {
		return "<nil>"
	}
	return c.model
}

// Version returns the remote host's software version.
func (c *Client) Version() string {
	if c == nil {
		return "<nil>"
	}
	return c.version
}

// Run creates a new SSH session, starts a remote shell, and runs the
// specified commands on the remote host.
func (c *Client) Run(cmds ...string) ([]byte, error) {
	// create a new session
	session, err := c.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	// create a pipe to the remote device's standard input
	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("could not create pipe to remote standard input: %v", err)
	}
	defer stdin.Close()

	// copy the remote device's standard output to a buffer
	var buf bytes.Buffer
	session.Stdout = &buf

	// start the remote shell
	if err := session.Shell(); err != nil {
		return nil, fmt.Errorf("could not start remote shell: %v", err)
	}

	// run the commands
	for _, cmd := range cmds {
		if _, err := stdin.Write([]byte(cmd + "\n")); err != nil {
			return nil, fmt.Errorf("failed to run %q: %v", cmd, err)
		}
	}

	// wait for the remote commands to exit or time out
	wait := make(chan error)
	go func() {
		wait <- session.Wait()
	}()
	select {
	case err := <-wait:
		if err != nil {
			switch v := err.(type) {
			case *ssh.ExitError:
				return nil, fmt.Errorf("session exited with status %d: %v", v.ExitStatus(), v)
			case *ssh.ExitMissingError:
				return nil, fmt.Errorf("session exited with no status: %v", v)
			default:
				return nil, fmt.Errorf("session failed to exit: %v", v)
			}
		}
		return buf.Bytes(), nil
	case <-time.After(Timeout):
		return nil, errors.New("session timed out")
	}
}

// String is the string representation of a client.
func (c *Client) String() string {
	if c == nil {
		return "<nil>"
	}
	return fmt.Sprintf("IP Addr: %s, Hostname: %s, Vendor: %s, OS: %s, Model: %s, Version: %s",
		c.addr, c.hostname, c.vendor, c.os, c.model, c.version)
}

// Close closes the SSH client connection to the remote host.
func (c *Client) Close() error {
	return c.client.Close()
}

// sysDescr gets the sysDescr from an ssh client.
func sysDescr(addr string) map[string]string {
	info := make(chan map[string]string)
	go getSysDescr(addr, info)

	// lookup hostname
	var hostname string
	names, err := net.LookupAddr(addr)
	if err == nil && len(names) > 0 {
		hostname = names[0]
	}
	m := <-info
	m["addr"] = addr
	m["hostname"] = hostname
	return m
}

// getSysDescr gets the sysDescr of a host through SNMP.
func getSysDescr(addr string, info chan<- map[string]string) {
	defer close(info)

	client, err := snmp.NewClient(addr, viper.GetString("snmp.community"), snmp.Version2c, 5)
	if err != nil {
		info <- map[string]string{"addr": "", "hostname": "", "vendor": "", "os": "", "model": "", "version": ""}
		return
	}
	defer client.Close()

	res, err := client.Get(".1.3.6.1.2.1.1.1.0")
	if err != nil {
		info <- map[string]string{"addr": "", "hostname": "", "vendor": "", "os": "", "model": "", "version": ""}
		return
	}
	var descr string
	for _, v := range res.Variables {
		if v.Type == snmp.OctetString {
			descr = v.Value.(string)
			break
		}
	}
	info <- parseSysDescr(descr)
}

const (
	// Cisco IOS, IOS XE, IOS XR, and NX-OS regexp strings
	ciscoModel    = `(([CATcat]{1,3}|[Nn]|[Mm]|[CGRcgr]{3})(\d{4}\w?|\d\w_\w*)|\w?\d*_rp)`
	ciscoSoftware = ciscoModel + `(-(\w*[Kk]9|Y|I)([-_]([WANwan-]*)?[Mm][Zz]?)?)`
	ciscoVersion  = `(Version (\(?(\d{1,2}|\w{1,2})\)?\.?)*)([[(].*[])])?(,?\s?)(RELEASE SOFTWARE (\(.*\)))?`

	// HPE Comware and Procurve
	hpeModel        = `(HP|HPE|ProCurve).*Switch\s?\w*,?`
	comwareVersion  = `Software\sVersion\s(\d{1,3}\.?)*,?\s?Release\s\d{4}`
	procurveVersion = `revision [A-Z]{1,2}(\.[0-9]{2,4})*,?\s?ROM [A-Z]{1,2}(\.[0-9]{2,4})*`
)

var (
	// Cisco
	modelCisco    = regexp.MustCompile(ciscoModel)
	softwareCisco = regexp.MustCompile(ciscoSoftware)
	versionCisco  = regexp.MustCompile(ciscoVersion)

	// Hewlett Packard
	modelHPE        = regexp.MustCompile(hpeModel)
	versionComware  = regexp.MustCompile(comwareVersion)
	versionProCurve = regexp.MustCompile(procurveVersion)
)

// parseSysDescr parses the sysDescr.0 OID string to gather device information.
func parseSysDescr(sysDescr string) map[string]string {
	m := map[string]string{
		"addr":     "",
		"hostname": "",
		"vendor":   "",
		"os":       "",
		"model":    "",
		"version":  "",
	}
	switch {
	case strings.Contains(sysDescr, "Cisco"):
		m["vendor"] = "CISCO"
		m["model"] = modelCisco.FindString(sysDescr)
		software := softwareCisco.FindString(sysDescr)
		version := versionCisco.FindString(sysDescr)
		v := strings.Replace(version, ",", "", 5)
		m["version"] = strings.TrimSpace(fmt.Sprintf("%s %s", software, v))
		switch {
		case strings.Contains(sysDescr, "IOS"):
			switch {
			case strings.Contains(sysDescr, "IOS XR"), strings.Contains(sysDescr, "IOS-XR"):
				m["os"] = "IOS XR"
			case strings.Contains(sysDescr, "IOS XE"), strings.Contains(sysDescr, "IOS-XE"):
				m["os"] = "IOS XE"
			default:
				m["os"] = "IOS"
			}
		case strings.Contains(sysDescr, "NX OS"), strings.Contains(sysDescr, "NX-OS"):
			m["os"] = "NX-OS"
		}
	case strings.Contains(sysDescr, "Hewlett Packard"),
		strings.Contains(sysDescr, "HP"),
		strings.Contains(sysDescr, "ProCurve"):
		m["vendor"] = "HP"
		m["model"] = modelHPE.FindString(sysDescr)
		switch {
		case strings.Contains(sysDescr, "Comware"):
			m["os"] = "Comware"
			m["version"] = versionComware.FindString(sysDescr)
		case strings.Contains(sysDescr, "ProCurve"):
			m["os"] = "ProCurve"
			m["version"] = versionProCurve.FindString(sysDescr)
		}
	}
	return m
}
