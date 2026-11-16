package gpu

import "testing"

func TestCompositeFullReason(t *testing.T) {
	tests := []struct {
		name         string
		sampleCount  int
		hasDamage    bool
		viewRendered bool
		want         string
	}{
		{name: "no damage", sampleCount: 1, hasDamage: false, want: "no_damage"},
		{name: "msaa drops damage", sampleCount: 4, hasDamage: true, viewRendered: true, want: "msaa"},
		{name: "msaa wins over fresh", sampleCount: 4, hasDamage: true, want: "msaa"},
		{name: "fresh target", sampleCount: 1, hasDamage: true, want: "fresh_target"},
		{name: "damage scissored", sampleCount: 1, hasDamage: true, viewRendered: true, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := compositeFullReason(tc.sampleCount, tc.hasDamage, tc.viewRendered)
			if got != tc.want {
				t.Fatalf("compositeFullReason(%d,%v,%v)=%q want %q",
					tc.sampleCount, tc.hasDamage, tc.viewRendered, got, tc.want)
			}
		})
	}
}
