package cmd

import (
	"bufio"
	"strings"
	"testing"
	"io"
	"fmt"
	"io/ioutil"
)

func TestRunInitCmd(t *testing.T) {

}

func TestInitCfg(t *testing.T) {

}

func TestIT(t *testing.T) {

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
