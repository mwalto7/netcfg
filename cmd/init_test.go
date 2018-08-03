package cmd

import (
	"bufio"
	"io/ioutil"
	"strings"
	"testing"
)

func TestRunInitCmd(t *testing.T) {

}

func TestInitCfg(t *testing.T) {

}

func TestInitIT(t *testing.T) {

}

func TestGetValue(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultVal interface{}
	}{
		{"empty", "", nil},
		{"default", "", "default"},
		{"new value", "new value", "default"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(test.input + string('\n')))
			got, err := getVal("Enter test value:", test.defaultVal, r, ioutil.Discard)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != test.input {
				t.Errorf("want %q, got %q", test.input, got)
			}
		})
	}
}
