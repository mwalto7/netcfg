# netcfg

[![Build Status](https://travis-ci.com/mwalto7/netcfg.svg?branch=master)](https://travis-ci.com/mwalto7/netcfg)
[![GoDoc](https://godoc.org/github.com/mwalto7/netcfg?status.svg)](https://godoc.org/github.com/mwalto7/netcfg)
[![Go Report Card](https://goreportcard.com/badge/github.com/mwalto7/netcfg)](https://goreportcard.com/report/github.com/mwalto7/netcfg)

## Overview

netcfg is a command line tool for fast, configurable network device 
configuration written in [Go](https://golang.org). It supports multi-host, 
multi-vendor configurations and is highly configurable with support for 
[YAML](http://yaml.org/spec/1.2/spec.html) and Go's 
[text templates](https://golang.org/pkg/text/template/).

## Getting Started

#### Install

```
go get -u github.com/mwalto7/netcfg
```

#### Global Config

Currently, netcfg uses SNMP version 2c to gather information about a device.
In order to allow this functionality, netcfg uses a global configuration file
for SNMP settings. The default config file is located at `~/.netcfg.yml` and
should have the following contents:

```yaml
# SNMP settings for `netcfg`. 

# .netcfg.yml
---
snmp:
  community: your-snmp-community
```

Support for SNMP version 3 is in the works.

## Configuration

netcfg uses YAML and Go's text templates to allow custom configurations
for network devices. The format for a netcfg configuration file is as follows:

```yaml
# config.yml
---
# file containing IP addresses of devices to configure
hosts: path/to/hosts_file

# username for device login
user: username

# password for device login (not required)
#
# `{{password}}` is a special function that will prompt
# for your password when the configuration file is run.
# It is not recommended to set `pass:` to your plain text
# password.
pass: {{password}}

# sequence of SSH private keys to use for device login
keys:
  - path/to/key1
  - path/to/key2
  # ...
  
# accept connections to these hosts only
#
# `accept` accepts two keywords: "all" and "known_hosts"
# "all" allows connections to any host, while "known_hosts"
# allows connections to only those hosts found in the
# OpenSSH known_hosts file (usually at ~/.ssh/known_hosts). 
accept : all # or known_hosts

# timeout is the time to wait to establish an SSH connection
#
# accepts the format <integer><unit>, i.e. 5s for 5 seconds,
# 25ms for 25 milliseconds, etc.
timeout: 10s

# aliases is a sequence of YAML aliases to be used throughout
# the configuration file. Useful for setting default command
# sets and making the file more modular and reusable.
aliases:
  # Each alias sequence item uses the same format as config
  # sequence items shown below.

# config is a sequence of configuration command sets.
config:

  # Each config sequence item may contain one or more of the
  # following options to have fine-grained control over what
  # specific hosts these commands apply to. The `cmds` key
  # is required. If no options are set, the `cmds` apply to
  # any host device.
  - addr    : 127.0.0.1
    hostname: localhost
    vendor  : cisco
    os      : ios
    model   : c2960s
    version : 15.0(2)SE10a
    cmds:
      - cmd1
      - cmd2
      # ...
```

Currently supported devices include Cisco IOS, IOS XE, and IOS XR, and HP ProCurve and Comware. 
More are to come in the future. If you would like a certain device to be supported, send the output of
`snmpget host 1.3.6.1.2.1.1.1.0` to dev.mwalto7@gmail.com and I will try to implement it.

## Commands

netcfg has two main commands: `init` and `run`. 

#### init

The `init` command initializes a new configuration file for use with the `run` command.

```
$ netcfg init --help

Quickly initialize a new configuration file with custom options.

Interactive mode is the easiest and recommended way to get started.
Pass the '--cat' flag to print the configuration file after initialization.

  netcfg init --it --cat

Usage:
  netcfg init [filename] [flags]

Examples:
If using more than one SSH key, separate the keys by commas inside
of the double quotes. If 'password' is true and 'keys' is not
empty, then password authentication will be used as a backup
for key authentication.

  # Disable password authentication and only use private key(s). 
  netcfg init --password=false --keys="$HOME/.ssh/"{key1,key2,...}

Timeout values must be formatted as '<number><unit>'. For example:

  netcfg init -t 5s   # 5 seconds
  netcfg init -t 25ms # 25 milliseconds
  netcfg init -t 2m   # 2 minutes

Flags:
  -a, --accept string        hosts to accept connections to (default "all")
      --cat                  print the initialized configuration file
  -d, --description string   description for this configuration
  -h, --help                 help for init
  -f, --hosts string         file of hosts to configure (default "hosts.txt")
      --it                   use interactive mode
      --keys strings         ssh keys to use for authentication
      --open                 open configuration file in editor (default vim)
      --password             use password authentication (default true)
  -t, --timeout duration     time to wait to establish connections (default 10s)
  -u, --username string      username for host login (default "mason")

Global Flags:
      --config string   config file (default is $HOME/.netcfg.yml)
```

#### run

The run command executes a configuration file and configures the specified hosts.

```
$ netcfg run --help

Run a configuration

Usage:
  netcfg run file [flags]

Flags:
  -c, --community string   SNMP v2c community string (default "public")
      --dry-run            test a configuration without configuring any hosts
  -h, --help               help for run
  -t, --template string    template data to use in configuration file

Global Flags:
      --config string   config file (default is $HOME/.netcfg.yml)
```