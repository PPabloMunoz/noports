package cmd

import (
	"reflect"
	"testing"
)

func TestSetEnvVar(t *testing.T) {
	tests := []struct {
		name  string
		env   []string
		key   string
		value string
		want  []string
	}{
		{"append to empty", nil, "PORT", "3000", []string{"PORT=3000"}},
		{"append new key", []string{"A=1"}, "PORT", "3000", []string{"A=1", "PORT=3000"}},
		{"replace existing", []string{"PORT=1", "A=2"}, "PORT", "3000", []string{"PORT=3000", "A=2"}},
		{"prefix collision kept", []string{"PORTX=1"}, "PORT", "3000", []string{"PORTX=1", "PORT=3000"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := setEnvVar(tt.env, tt.key, tt.value); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("setEnvVar = %q, want %q", got, tt.want)
			}
		})
	}
}
