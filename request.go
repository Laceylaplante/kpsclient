package kps

import (
	"encoding/xml"
	"fmt"
	"strings"
)

const bodyNS = "http://kps.nvi.gov.tr/2025/08/01"

// BuildTumKutukBody, TumKutukDogrulamaServisi için <Sorgula> gövdesini üretir.
func BuildTumKutukBody(r QueryRequest) string {
	esc := func(s string) string {
		var b strings.Builder
		_ = xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	zeroIfEmpty := func(s string) string {
		if strings.TrimSpace(s) == "" {
			return "0"
		}
		return s
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<Sorgula xmlns="%s" xmlns:i="http://www.w3.org/2001/XMLSchema-instance">`, bodyNS))
	sb.WriteString(`<kriterListesi><TumKutukDogrulamaSorguKriteri>`)

	sb.WriteString(fmt.Sprintf(`<Ad>%s</Ad>`, esc(r.FirstName)))
	sb.WriteString(fmt.Sprintf(`<DogumAy>%s</DogumAy>`, esc(zeroIfEmpty(r.BirthMonth))))
	sb.WriteString(fmt.Sprintf(`<DogumGun>%s</DogumGun>`, esc(zeroIfEmpty(r.BirthDay))))
	sb.WriteString(fmt.Sprintf(`<DogumYil>%s</DogumYil>`, esc(r.BirthYear)))
	sb.WriteString(fmt.Sprintf(`<KimlikNo>%s</KimlikNo>`, esc(r.TCNo)))
	sb.WriteString(fmt.Sprintf(`<Soyad>%s</Soyad>`, esc(r.LastName)))

	//TCKKSeriNo) opsiyonel

	sb.WriteString(`</TumKutukDogrulamaSorguKriteri></kriterListesi></Sorgula>`)
	return sb.String()
}
