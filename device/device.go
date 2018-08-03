package device

import (
	"fmt"
	"io"
	"io/ioutil"
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
	*ssh.Client     // underlying SSH client connection
	addr     string // IP address of the device
	hostname string // hostname of the device
	vendor   string // vendor of the device
	os       string // operating system of the device
	model    string // model of the device
	version  string // software version of the device
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
		Client:   client,
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
	return c.addr
}

// Hostname returns the remote host's hostname.
func (c *Client) Hostname() string {
	return c.hostname
}

// Vendor returns the remote host's vendor.
func (c *Client) Vendor() string {
	return c.vendor
}

// OS returns the remote host's operating system.
func (c *Client) OS() string {
	return c.os
}

// Model returns the remote host's model.
func (c *Client) Model() string {
	return c.model
}

// Version returns the remote host's software version.
func (c *Client) Version() string {
	return c.version
}

// Run creates a new SSH session, starts a remote shell, and runs the
// specified commands on the remote host.
func (c *Client) Run(cmds ...string) ([]byte, error) {
	session, err := c.Client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	stdin, stdout, stderr, err := pipeIO(session)
	if err != nil {
		return nil, err
	}
	defer stdin.Close()

	if err := session.Shell(); err != nil {
		return nil, err
	}
	for _, cmd := range cmds {
		_, err := io.WriteString(stdin, fmt.Sprintf("%s\n", cmd))
		if err != nil {
			return nil, err
		}
	}
	wait := make(chan error, 1)
	go func() {
		wait <- session.Wait()
	}()
	select {
	case err := <-wait:
		if err != nil {
			switch err.(type) {
			case *ssh.ExitError:
				// TODO: handle exit errors
			case *ssh.ExitMissingError:
				// TODO: handle missing exit errors
			default:
				return nil, err
			}
		}
		b, err := ioutil.ReadAll(io.MultiReader(stdout, stderr))
		if err != nil {
			return nil, err
		}
		return b, nil
	case <-time.After(Timeout):
		return nil, errors.New("session timed out")
	}
}

// pipeIO creates pipes to the remote shell's standard input, standard
// output, and standard error.
func pipeIO(session *ssh.Session) (stdin io.WriteCloser, stdout, stderr io.Reader, err error) {
	stdin, err = session.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err = session.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stderr, err = session.StderrPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	return
}

// String is the string representation of a client.
func (c *Client) String() string {
	return fmt.Sprintf("IP Addr: %s, Hostname: %s, Vendor: %s, OS: %s, Model: %s, Version: %s",
		c.addr, c.hostname, c.vendor, c.os, c.model, c.version)
}

// Close closes the SSH client connection to the remote host.
func (c *Client) Close() error {
	return c.Client.Close()
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
		m["model"] = regexp.MustCompile(`([Cc]|[Cc][Aa][Tt]|[Nn]|[Mm]|[Cc][Gg][Rr])(\d{4}\w?|\d\w_\w*)|(\w?\d*_rp)|ASR\dK`).FindString(sysDescr)
		software := regexp.MustCompile(`([CATcat]*|[Nn]|[Mm]|[CGRcgr]*|\w?\d*_rp)(\d\w_\w*|\d{4}\w?)?(-(\w*[Kk]9|Y|I)([-_]([WANwan-]*)?[Mm][Zz]?)?)`).FindString(sysDescr)
		version := regexp.MustCompile(`(Version (\(?(\d{1,2}|\w{1,2})\)?\.?)*)([[(].*[])])?(,?\s?)(RELEASE SOFTWARE (\(.*\)))?`).FindString(sysDescr)
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
	case strings.Contains(sysDescr, "Hewlett Packard"), strings.Contains(sysDescr, "HP"), strings.Contains(sysDescr, "ProCurve"):
		m["vendor"] = "HP"
		m["model"] = regexp.MustCompile(`(HP|HPE|ProCurve).*Switch\s?\w*,?`).FindString(sysDescr)
		switch {
		case strings.Contains(sysDescr, "Comware"):
			m["os"] = "Comware"
			m["version"] = regexp.MustCompile(`Software\sVersion\s(\d{1,3}\.?)*,?\s?Release\s\d{4}`).FindString(sysDescr)
		case strings.Contains(sysDescr, "ProCurve"):
			m["os"] = "ProCurve"
			m["version"] = regexp.MustCompile(`revision [A-Z]{1,2}(\.[0-9]{2,4})*,?\s?ROM [A-Z]{1,2}(\.[0-9]{2,4})*`).FindString(sysDescr)
		}
	}
	return m
}
