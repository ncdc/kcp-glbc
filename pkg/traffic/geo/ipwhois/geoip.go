package ipwhois

import (
	"embed"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

//go:embed *.json
var fs embed.FS

const (
	DefaultContinentCode  = "NA"
)

type GeoIP struct {
	IP            string  `json:"ip"`
	Success       bool    `json:"success"`
	Type          string  `json:"type"`
	Continent     string  `json:"continent"`
	ContinentCode string  `json:"continent_code"`
	Country       string  `json:"country"`
	CountryCode   string  `json:"country_code"`
	Region        string  `json:"region"`
	RegionCode    string  `json:"region_code"`
	City          string  `json:"city"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	IsEu          bool    `json:"is_eu"`
	Postal        string  `json:"postal"`
	CallingCode   string  `json:"calling_code"`
	Capital       string  `json:"capital"`
	Borders       string  `json:"borders"`
	Flag          struct {
		Img          string `json:"img"`
		Emoji        string `json:"emoji"`
		EmojiUnicode string `json:"emoji_unicode"`
	} `json:"flag"`
	Connection struct {
		Asn    int    `json:"asn"`
		Org    string `json:"org"`
		Isp    string `json:"isp"`
		Domain string `json:"domain"`
	} `json:"connection"`
	Timezone struct {
		ID          string `json:"id"`
		Abbr        string `json:"abbr"`
		IsDst       bool   `json:"is_dst"`
		Offset      int    `json:"offset"`
		Utc         string `json:"utc"`
		CurrentTime string `json:"current_time"`
	} `json:"timezone"`
}

func GetContinentCodeForIp(ip string) string {

	data := getLocalDataFor(ip)
	if data == nil {
		data = getRemoteDataFor(ip)
	}
	if data == nil {
		return DefaultContinentCode
	}

	geoip := GeoIP{}
	err := json.Unmarshal(data, &geoip)
	if err != nil {
		return DefaultContinentCode
	}
	if geoip.Success {
		return geoip.ContinentCode
	}
	return DefaultContinentCode
}

func getLocalDataFor(ip string) []byte {
	data, err := fs.ReadFile(ip + ".json")
	if err != nil {
		return nil
	}
	return data
}

func getRemoteDataFor(ip string) []byte {
	response, err := http.Get("http://ipwho.is/" + ip)
	if err != nil {
		return nil
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil
	}

	return data
}

