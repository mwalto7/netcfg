// Copyright © 2018 Mason Walton <dev.mwalto7@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/mwalto7/netcfg/config"
	"github.com/mwalto7/netcfg/device"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var (
	dryRun  bool
	tmpl    string
	workers int
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a configuration",
	RunE:  runCmdRunE,
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "test a configuration without configuring any hosts")
	runCmd.Flags().StringVarP(&tmpl, "template", "t", "", "template data to use in configuration file")
	runCmd.Flags().StringP("community", "c", "public", "SNMP v2c community string")
	runCmd.Flags().IntVarP(&workers, "workers", "w", 1, "number of workers to run, more = faster")
}

// runCmdRunE is the function fun for the `runCmd`.
func runCmdRunE(_ *cobra.Command, args []string) error {
	var cfgData, tmplData string

	b, err := ioutil.ReadFile(args[0])
	if err != nil {
		return err
	}
	cfgData = string(b)

	if tmpl != "" {
		b, err := ioutil.ReadFile(tmpl)
		if err != nil {
			return err
		}
		tmplData = string(b)
	}

	cfg, err := config.New("cfg").Template(tmplData).Parse(cfgData)
	if err != nil {
		return err
	}
	if dryRun {
		return dryRunCfg(cfg)
	}
	device.Timeout = cfg.Timeout
	return runCfg(cfg)
}

// dryRunCfg prints out the parsed config and all command sets.
func dryRunCfg(cfg *config.Config) error {
	fmt.Println(cfg.Name())
	cfgCmds, err := config.MapCmds(cfg)
	if err != nil {
		return err
	}
	for vendor, cmdSet := range cfgCmds {
		fmt.Printf("[%s]\n", vendor)
		for _, cmd := range cmdSet {
			fmt.Println(cmd)
		}
		fmt.Println()
	}
	fmt.Println(cfg.String())
	return nil
}

// result represents a configuration result.
type result struct {
	host string // host configured
	out  []byte // output of configuration
	err  error  // error from configuration
}

// runCfg is the `runCmd`'s main function.
func runCfg(cfg *config.Config) error {
	// read hosts file from user config
	hostsData, err := ioutil.ReadFile(cfg.Hosts)
	if err != nil {
		return fmt.Errorf("run: failed to read %s: %v", cfg.Hosts, err)
	}

	// scan hosts line-by-line from hosts file
	var hosts []string
	s := bufio.NewScanner(bytes.NewReader(hostsData))
	for s.Scan() {
		line := s.Text()
		if line != "" {
			hosts = append(hosts, line)
		}
	}
	if err := s.Err(); err != nil {
		return fmt.Errorf("run: error scanning %s: %v", cfg.Hosts, err)
	}
	if len(hosts) == 0 {
		return errors.New("run: no hosts to configure")
	}

	// convert user config commands to map
	cfgCmds, err := config.MapCmds(cfg)
	if err != nil {
		return fmt.Errorf("run: could not map commands: %v", err)
	}
	if len(cfgCmds) == 0 {
		return errors.New("run: no configuration commands to run")
	}

	// the network devices to configure and their configuration results
	devices := make(chan string, len(hosts))
	results := make(chan result, len(hosts))

	// start workers
	var wg sync.WaitGroup
	numWorkers := runtime.NumCPU() * workers
	wg.Add(numWorkers)
	for w := 0; w < numWorkers; w++ {
		cfg := cfg
		cmds := cfgCmds
		go configure(cmds, cfg, devices, results, &wg)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	// send jobs to the workers
	for _, host := range hosts {
		devices <- host
	}
	close(devices)

	// read the results
	for i := 0; i < len(hosts); i++ {
		res, ok := <-results
		if !ok {
			return errors.New("run: error reading results, nil channel")
		}
		if res.err != nil {
			fmt.Fprintf(os.Stderr, "%s error: %v\n", res.host, res.err)
			continue
		}
		fmt.Printf("%s\n%s\n%s\n", res.host, res.out, strings.Repeat("-", 50))
	}
	return nil
}

// configure is a worker that creates a client connection to each host in `devices`
// then returns the open client connection.
func configure(cfgCmds map[string][]string, cfg *config.Config, devices <-chan string, results chan<- result, wg *sync.WaitGroup) {
	defer wg.Done()

	for host := range devices {
		cfg := cfg
		cfgCmds := cfgCmds

		clientCfg := &ssh.ClientConfig{
			User:            cfg.User,
			Auth:            []ssh.AuthMethod{ssh.Password(cfg.Pass)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         cfg.Timeout,
		}
		clientCfg.SetDefaults()
		clientCfg.Ciphers = append(clientCfg.Ciphers, "aes128-cbc", "aes256-cbc", "3des-cbc", "des-cbc", "aes192-cbc")

		// establish client connection to remote device
		client, err := device.Dial(host, "22", clientCfg)
		if err != nil {
			results <- result{host, nil, fmt.Errorf("failed to dial %s: %v", host, err)}
			continue
		}

		// choose the right command set to send to the remote device
		cmds := make([]string, 0)
		for k, v := range cfgCmds {
			m := make(map[string]string)
			for _, info := range strings.Split(k, ",") {
				opts := strings.Split(info, ":")
				opts[0] = strings.TrimSpace(opts[0])
				opts[1] = strings.Replace(opts[1], `"`, "", -1)
				m[opts[0]] = strings.TrimSpace(strings.ToLower(opts[1]))
			}
			if m["IP Addr"] != "" && m["IP Addr"] != strings.ToLower(client.Addr()) ||
				m["Hostname"] != "" && m["Hostname"] != strings.ToLower(client.Hostname()) ||
				m["Vendor"] != "" && m["Vendor"] != strings.ToLower(client.Vendor()) ||
				m["OS"] != "" && m["OS"] != strings.ToLower(client.OS()) ||
				m["Model"] != "" && m["Model"] != strings.ToLower(client.Model()) ||
				m["Version"] != "" && m["Version"] != strings.ToLower(client.Version()) {
				continue
			}
			cmds = v
		}
		if genericCmds, ok := cfgCmds["generic"]; ok && len(cmds) == 0 {
			cmds = genericCmds
		}
		if len(cmds) == 0 {
			results <- result{host, nil, fmt.Errorf("no commands to run")}
			client.Close()
			continue
		}

		// run the commands on the remote device
		out, err := client.Run(cmds...)
		if err != nil {
			results <- result{host, nil, fmt.Errorf("failed to run commands: %v", err)}
			client.Close()
			continue
		}
		results <- result{client.String(), out, nil}
		client.Close()
	}
}
