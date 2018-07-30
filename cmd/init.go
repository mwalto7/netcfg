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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
)

const (
	initLong = `Quickly initialize a new configuration file with custom options.

Interactive mode is the easiest and recommended way to get started.
Pass the '--cat' flag to print the configuration file after initialization.

  netcfg init --it --cat`

	initExample = `If using more than one SSH key, separate the keys by commas inside
of the double quotes. If 'password' is true and 'keys' is not
empty, then password authentication will be used as a backup
for key authentication.

  # Disable password authentication and only use private key(s). 
  netcfg init --password=false --keys="$HOME/.ssh/"{key1,key2,...}

Timeout values must be formatted as '<number><unit>'. For example:

  netcfg init -t 5s   # 5 seconds
  netcfg init -t 25ms # 25 milliseconds
  netcfg init -t 2m   # 2 minutes`

	initTmpl = `
{{- if ne .description "" -}}
  {{- printf "# %s\n\n" .description}}
{{- end -}}
# {{.filename}}
---
hosts: {{.hosts}}
user: {{.user}}
{{if .pass -}}
pass: {{printf "%s\n" "{{password}}"}}
{{- end}}
{{- if ne (len .keys) 0 -}}
keys:
  {{range .keys -}}
    {{- printf "  - %s\n" . -}}
  {{- end -}}
{{- end -}}
accept: {{.accept}}
timeout: {{.timeout}}

aliases:
  # Add your aliases here

config:
  # Add your configurations here.
`
)

var (
	cat         bool          // print the cfg file
	open        bool          // open the config after init
	interactive bool          // init using interactive mode
	description string        // cfg file description
	filename    string        // name of the configuration file
	hosts       string        // file of hosts to configure
	user        string        // username for host login
	keys        []string      // ssh private keys
	pass        bool          // use password authentication
	accept      string        // accept connections to these hosts
	timeout     time.Duration // ssh connection timeout
)

// initCmd represents the new command.
var initCmd = &cobra.Command{
	Use:     "init [filename]",
	Short:   "Initialize a new configuration file",
	Long:    initLong,
	Args:    cobra.MaximumNArgs(1),
	Example: initExample,
	RunE:    runInitCmd,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&cat, "cat", false, "print the initialized configuration file")
	initCmd.Flags().BoolVar(&open, "open", false, "open configuration file in editor (default vim)")
	initCmd.Flags().BoolVar(&interactive, "it", false, "use interactive mode")
	initCmd.Flags().StringVarP(&description, "description", "d", "", "description for this configuration")
	initCmd.Flags().StringVarP(&hosts, "hosts", "f", "hosts.txt", "file of hosts to configure")
	initCmd.Flags().StringVarP(&user, "username", "u", os.Getenv("USER"), "username for host login")
	initCmd.Flags().BoolVar(&pass, "password", true, "use password authentication")
	initCmd.Flags().StringSliceVar(&keys, "keys", nil, "ssh keys to use for authentication")
	initCmd.Flags().StringVarP(&accept, "accept", "a", "all", "hosts to accept connections to")
	initCmd.Flags().DurationVarP(&timeout, "timeout", "t", 10*time.Second, "time to wait to establish connections")
}

// runInitCmd is the main function passed to `RunE` of the `initCmd`.
func runInitCmd(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		filename = "config.yml"
	} else {
		filename = args[0]
	}
	if !strings.HasSuffix(filename, ".yml") {
		if !strings.HasSuffix(filename, ".yaml") {
			filename += ".yml"
		}
	}
	data := map[string]interface{}{
		"description": description,
		"filename":    filename,
		"hosts":       hosts,
		"user":        user,
		"pass":        pass,
		"keys":        keys,
		"accept":      accept,
		"timeout":     timeout,
	}
	if interactive {
		itData, err := initIT(data)
		if err != nil {
			return err
		}
		data = itData
	}
	return initCfg(initTmpl, data)
}

// initCfg initializes a new configuration file with the specified options.
func initCfg(src string, data map[string]interface{}) error {
	f, err := os.Create(data["filename"].(string))
	if err != nil {
		return err
	}
	defer f.Close()

	tmpl, err := template.New("init").Parse(src)
	if err != nil {
		return err
	}
	if err := tmpl.Execute(f, &data); err != nil {
		return err
	}
	if cat {
		b, err := ioutil.ReadFile(f.Name())
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "\n%s", string(b))
	}
	if open {
		cmd := exec.Command("vim", f.Name())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Successfully created: %s\n", f.Name())
	fmt.Fprintf(os.Stderr, "Test it with 'netcfg run %s --dry-run'\n", f.Name())
	return nil
}

// initIT enables interactive mode for setting options for a new configuration file.
func initIT(data map[string]interface{}) (map[string]interface{}, error) {
	itData := make(map[string]interface{}, len(data))
	r := bufio.NewReader(os.Stdin)
	for k, v := range data {
		var prompt string
		switch val := v.(type) {
		case string:
			switch k {
			case "description":
				prompt = "Enter a description"
			case "filename":
				prompt = "Config file name"
			case "hosts":
				prompt = "Hosts file"
			case "user":
				prompt = "Username"
			case "accept":
			acceptPrompt:
				for {
					itVal, err := getVal(r, os.Stderr, "Allow connections to", val)
					if err != nil {
						return nil, err
					}
					switch itVal {
					case "":
						itData[k] = val
						break acceptPrompt
					case "all", "known_hosts":
						itData[k] = itVal
						break acceptPrompt
					default:
						fmt.Fprintf(os.Stderr, "expected all or known_hosts, got %q\n", itVal)
						continue
					}
				}
				continue
			}
			itVal, err := getVal(r, os.Stderr, prompt, val)
			if err != nil {
				return nil, err
			}
			if itVal == "" {
				itData[k] = val
			} else {
				if k == "filename" && !strings.HasSuffix(itVal, ".yml") {
					if !strings.HasSuffix(itVal, ".yaml") {
						itVal += ".yml"
					}
				}
				itData[k] = itVal
			}
		case bool:
		pwprompt:
			for {
				itVal, err := getVal(r, os.Stderr, "Use password authentication?", val)
				if err != nil {
					return nil, err
				}
				switch strings.ToLower(itVal) {
				case "":
					itData[k] = val
					break pwprompt
				case "1", "y", "yes", "t", "true":
					itData[k] = true
					break pwprompt
				case "0", "n", "no", "f", "false":
					itData[k] = false
					break pwprompt
				default:
					fmt.Fprintf(os.Stderr, "Expected boolean, got %q\n", itVal)
					continue
				}
			}
		case time.Duration:
		tprompt:
			for {
				itVal, err := getVal(r, os.Stderr, "Connection timeout", val.String())
				if err != nil {
					return nil, err
				}
				if itVal == "" {
					itData[k] = val
					break tprompt
				} else {
					itTimeout, err := time.ParseDuration(itVal)
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						continue
					}
					itData[k] = itTimeout
					break tprompt
				}
			}
		case []string:
			itVal, err := getVal(r, os.Stderr, "Enter any SSH keys (comma separated)", keys)
			if err != nil {
				return nil, err
			}
			if itVal == "" {
				itData[k] = val
			} else {
				keys := strings.Split(itVal, ",")
				for i := range keys {
					keys[i] = strings.TrimSpace(keys[i])
				}
				itData[k] = keys
			}
		}
	}
	return itData, nil
}

// getVal prompts the user for a value
func getVal(r *bufio.Reader, w io.Writer, s string, v interface{}) (string, error) {
	str := fmt.Sprintf("%v", v)
	if v == nil || str == "" || str == "[]" {
		fmt.Fprintf(w, "%s: ", s)
	} else {
		fmt.Fprintf(w, "%s (%v): ", s, v)
	}
	val, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.TrimRight(val, string('\n'))), nil
}
