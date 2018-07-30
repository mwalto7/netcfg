package cmd

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

const (
	noError  = true
	hasError = false
)

func TestGetValue(t *testing.T) {
	tests := []struct {
		name   string
		file   string
		prompt string
		v      interface{}
		want   string
		ok     bool
	}{
		{"empty", "empty", "", nil, "", noError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f, err := ioutil.TempFile("", test.file)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			got, err := getVal(bufio.NewReader(f), ioutil.Discard, test.prompt, test.v)
			switch {
			case (err == nil || err == io.EOF) && !test.ok:
				t.Errorf("expected error, got none")
			case err != nil && err != io.EOF && test.ok:
				t.Errorf("unexpected error: %v", err)
			case err != nil && err != io.EOF && !test.ok:
				t.Logf("got expected error: %v", err)
				return
			}
			if got != test.want {
				t.Fatalf("expected %s, got %s", test.want, got)
			}
			if err := os.Remove(f.Name()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
