package dns

const (
	TargetTypeHost = "HOST"
	TargetTypeIP   = "IP"
)

type Target struct {
	TargetType string
	Value      []string
	TargetMeta TargetMeta
}

type TargetMeta struct {
	Geo GeoMeta
}

type GeoMeta struct {
	Continent     string `json:"continent"`
	ContinentCode string `json:"continent_code"`
	Country       string `json:"country"`
	CountryCode   string `json:"country_code"`
	Region        string `json:"region"`
	RegionCode    string `json:"region_code"`
	City          string `json:"city"`
}
