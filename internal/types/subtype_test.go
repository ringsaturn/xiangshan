package types

import "testing"

func TestSubtypeFromString(t *testing.T) {
	cases := []struct {
		input   string
		want    Subtype
		wantErr bool
	}{
		{"country", SubtypeCountry, false},
		{"dependency", SubtypeDependency, false},
		{"macroregion", SubtypeMacroRegion, false},
		{"region", SubtypeRegion, false},
		{"macrocounty", SubtypeMacroCounty, false},
		{"county", SubtypeCounty, false},
		{"localadmin", SubtypeLocalAdmin, false},
		{"locality", SubtypeLocality, false},
		{"", 0, true},
		{"COUNTRY", 0, true},
	}
	for _, c := range cases {
		got, err := SubtypeFromString(c.input)
		if (err != nil) != c.wantErr {
			t.Errorf("SubtypeFromString(%q) error = %v, wantErr %v", c.input, err, c.wantErr)
		}
		if !c.wantErr && got != c.want {
			t.Errorf("SubtypeFromString(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}
