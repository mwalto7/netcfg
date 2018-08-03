package cmd

import (
	"bufio"
	"strings"
	"testing"
	"io"
	"fmt"
	"io/ioutil"
	"os"
)

func TestRunInitCmd(t *testing.T) {

}

func TestInitCfg(t *testing.T) {

}

func TestIT(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		vals []string
		ok   bool
	}{
		{"nil", nil, nil, false},
		{"empty", make(map[string]interface{}), make([]string, 0), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := it(test.data, os.Stdin)
			switch {
			case err != nil && test.ok:
				t.Errorf("got unexpected error: %v", err)
			case err == nil && !test.ok:
				t.Errorf("expected error, got none")
			case err != nil && !test.ok:
				t.Logf("got expected error: %v", err)
			}
			t.Log(test.data)
		})
	}
}

func TestGetValue(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultVal interface{}
	}{
		{"nil", "", nil},
		{"empty", "", ""},
		{"default string", "", "default"},
		{"default bool", "", true},
		{"default slice", "", make([]string, 0)},
		{"override string", "new", "default"},
		{"override bool", "false", true},
		{"override slice", "key1,key2,key3", make([]string, 0)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			prompt := "Enter test value"
			r := bufio.NewReader(strings.NewReader(test.input + string('\n')))

			var input string
			pr, pw := io.Pipe()
			go func(pw *io.PipeWriter, t *testing.T) {
				got, err := getVal(prompt, test.defaultVal, r, pw)
				if err != nil {
					pw.CloseWithError(err)
					t.Errorf("unexpected error: %v", err)
					return
				}
				input = got
				pw.Close()
			}(pw, t)

			b, err := ioutil.ReadAll(pr)
			if err != nil {
				t.Fatal(err)
			}

			def := fmt.Sprintf("%v", test.defaultVal)
			if test.defaultVal == nil || def == "" || def == "[]" {
				p := fmt.Sprintf("%s: ", prompt)
				if string(b) != p {
					t.Fatalf("want %s, got %s", p, string(b))
				}
			} else {
				p := fmt.Sprintf("%s (%s): ", prompt, def)
				if string(b) != p {
					t.Fatalf("want %s, got %s", p, string(b))
				}
			}
			if input != test.input {
				t.Errorf("want %q, got %q", test.input, input)
			}
		})
	}
}
