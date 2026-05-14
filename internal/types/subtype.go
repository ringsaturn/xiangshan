package types

import "fmt"

type Subtype uint8

const (
	SubtypeCountry Subtype = iota
	SubtypeDependency
	SubtypeMacroRegion
	SubtypeRegion
	SubtypeMacroCounty
	SubtypeCounty
	SubtypeLocalAdmin
	SubtypeLocality
)

func SubtypeFromString(s string) (Subtype, error) {
	switch s {
	case "country":
		return SubtypeCountry, nil
	case "dependency":
		return SubtypeDependency, nil
	case "macroregion":
		return SubtypeMacroRegion, nil
	case "region":
		return SubtypeRegion, nil
	case "macrocounty":
		return SubtypeMacroCounty, nil
	case "county":
		return SubtypeCounty, nil
	case "localadmin":
		return SubtypeLocalAdmin, nil
	case "locality":
		return SubtypeLocality, nil
	default:
		return 0, fmt.Errorf("xiangshan: unknown subtype %q", s)
	}
}

func (s Subtype) String() string {
	switch s {
	case SubtypeCountry:
		return "country"
	case SubtypeDependency:
		return "dependency"
	case SubtypeMacroRegion:
		return "macroregion"
	case SubtypeRegion:
		return "region"
	case SubtypeMacroCounty:
		return "macrocounty"
	case SubtypeCounty:
		return "county"
	case SubtypeLocalAdmin:
		return "localadmin"
	case SubtypeLocality:
		return "locality"
	default:
		return fmt.Sprintf("Subtype(%d)", s)
	}
}
