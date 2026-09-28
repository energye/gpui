//go:build linux && !(js && wasm)

package gles

import (
	"testing"
)

func TestPrimeEnvForPower(t *testing.T) {
	cases := []struct {
		name   string
		power  string
		env    map[string]string
		want   [][2]string
		wantOK bool
	}{
		{
			name:   "high selects discrete",
			power:  "high",
			want:   [][2]string{{"DRI_PRIME", "1"}},
			wantOK: true,
		},
		{
			name:   "discrete alias",
			power:  "discrete",
			want:   [][2]string{{"DRI_PRIME", "1"}},
			wantOK: true,
		},
		{name: "low keeps integrated", power: "low", wantOK: false},
		{name: "unset keeps integrated", power: "", wantOK: false},
		{
			name:   "explicit prime wins",
			power:  "high",
			env:    map[string]string{"DRI_PRIME": "0"},
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GPUI_POWER", tc.power)
			if v, ok := tc.env["DRI_PRIME"]; ok {
				t.Setenv("DRI_PRIME", v)
			} else {
				t.Setenv("DRI_PRIME", "")
			}
			got, ok := primeEnvForPower()
			if ok != tc.wantOK {
				t.Fatalf("primeEnvForPower() ok=%v want %v (got %v)", ok, tc.wantOK, got)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("primeEnvForPower() = %v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("primeEnvForPower()[%d] = %v want %v", i, got[i], tc.want[i])
				}
			}
		})
	}
}
