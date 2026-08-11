package dnd5e

import "testing"

func TestNormalizeSize(t *testing.T) {
	tests := []struct {
		name string
		in   Size
		want Size
	}{
		{name: "known lowercase", in: SizeLarge, want: SizeLarge},
		{name: "known mixed case", in: "Small", want: SizeSmall},
		{name: "omitted defaults to medium", want: SizeMedium},
		{name: "unknown defaults to medium", in: "colossal", want: SizeMedium},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeSize(tt.in); got != tt.want {
				t.Errorf("NormalizeSize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
