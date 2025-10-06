package kps

import (
	"strconv"
	"strings"

	"github.com/antchfx/xmlquery"
)

func ParseTumKutukResponse(respXML string) (Result, error) {
	doc, err := xmlquery.Parse(strings.NewReader(respXML))
	if err != nil {
		return Result{
			Status:   false,
			Code:     2,
			Aciklama: "XML parse hatası",
			Raw:      respXML,
		}, err
	}

	get := func(base *xmlquery.Node, xp string) string {
		if base == nil || xp == "" {
			return ""
		}
		n := xmlquery.FindOne(base, xp+"[not(@*[local-name()='nil']='true')]")
		if n == nil {
			return ""
		}
		return strings.TrimSpace(n.InnerText())
	}
	pad2 := func(s string) string {
		s = strings.TrimSpace(s)
		if s == "" {
			return ""
		}
		if len(s) == 1 {
			return "0" + s
		}
		return s
	}
	joinDate := func(y, m, d string) string {
		y, m, d = strings.TrimSpace(y), strings.TrimSpace(m), strings.TrimSpace(d)
		switch {
		case y == "":
			return ""
		case m == "" && d == "":
			return y
		case d == "":
			return y + "-" + pad2(m)
		default:
			return y + "-" + pad2(m) + "-" + pad2(d)
		}
	}

	prefer := strings.ToLower(strings.ReplaceAll(
		get(doc, "//*[local-name()='DoluBilesenler']/*[contains(local-name(), 'DogrulaServisDoluBilesen')]"),
		" ", "",
	))

	type bucket struct {
		path   string
		person Person
		name   string
	}

	tc := bucket{
		path:   "//*[local-name()='TCVatandasiKisiKutukleri' and not(@*[local-name()='nil']='true')]/*[local-name()='KisiBilgisi']",
		person: PersonTC,
		name:   "tc",
	}
	mavi := bucket{
		path:   "//*[local-name()='MaviKartliKisiKutukleri' and not(@*[local-name()='nil']='true')]/*[local-name()='KisiBilgisi']",
		person: PersonMavi,
		name:   "mavi",
	}
	yab := bucket{
		path:   "//*[local-name()='YabanciKisiKutukleri' and not(@*[local-name()='nil']='true')]/*[local-name()='KisiBilgisi']",
		person: PersonYab,
		name:   "yabanci",
	}

	var buckets []bucket
	switch {
	case strings.Contains(prefer, "yabanci"):
		buckets = []bucket{yab, tc, mavi}
	case strings.Contains(prefer, "tckisi"):
		buckets = []bucket{tc, yab, mavi}
	case strings.Contains(prefer, "mavikart"):
		buckets = []bucket{mavi, tc, yab}
	default:
		buckets = []bucket{tc, mavi, yab}
	}

	for _, b := range buckets {
		kisi := xmlquery.FindOne(doc, b.path)
		if kisi == nil {
			continue
		}

		kodStr := get(kisi, ".//*[local-name()='DurumBilgisi']/*[local-name()='Durum']/*[local-name()='Kod']")
		if kodStr == "" {
			kodStr = get(kisi, ".//*[local-name()='Durum']/*[local-name()='Kod']")
		}
		aciklama := get(kisi, ".//*[local-name()='DurumBilgisi']/*[local-name()='Durum']/*[local-name()='Aciklama']")
		if aciklama == "" {
			aciklama = get(kisi, ".//*[local-name()='Durum']/*[local-name()='Aciklama']")
		}

		if kodStr == "" {
			continue
		}

		code := 0
		if v, err := strconv.Atoi(kodStr); err == nil && v >= 0 {
			code = v
		}

		extra := map[string]string{}

		kimlik := get(kisi, ".//*[local-name()='TCKimlikNo']")
		if kimlik == "" {
			kimlik = get(kisi, ".//*[local-name()='KimlikNo']")
		}
		if kimlik != "" {
			extra["KimlikNo"] = kimlik
		}

		if ad := get(kisi, ".//*[local-name()='TemelBilgisi']/*[local-name()='Ad']"); ad != "" {
			extra["Ad"] = ad
		}
		if soyad := get(kisi, ".//*[local-name()='TemelBilgisi']/*[local-name()='Soyad']"); soyad != "" {
			extra["Soyad"] = soyad
		}
		if uyruk := get(kisi, ".//*[local-name()='TemelBilgisi']/*[local-name()='Uyruk']"); uyruk != "" {
			extra["Uyruk"] = uyruk
		}

		dy := get(kisi, ".//*[local-name()='DurumBilgisi']/*[local-name()='DogumTarih']/*[local-name()='Yil']")
		dm := get(kisi, ".//*[local-name()='DurumBilgisi']/*[local-name()='DogumTarih']/*[local-name()='Ay']")
		dd := get(kisi, ".//*[local-name()='DurumBilgisi']/*[local-name()='DogumTarih']/*[local-name()='Gun']")
		if dt := joinDate(dy, dm, dd); dt != "" {
			extra["DogumTarih"] = dt
		}

		oy := get(kisi, ".//*[local-name()='DurumBilgisi']/*[local-name()='OlumTarih']/*[local-name()='Yil']")
		om := get(kisi, ".//*[local-name()='DurumBilgisi']/*[local-name()='OlumTarih']/*[local-name()='Ay']")
		od := get(kisi, ".//*[local-name()='DurumBilgisi']/*[local-name()='OlumTarih']/*[local-name()='Gun']")
		if ot := joinDate(oy, om, od); ot != "" {
			extra["OlumTarih"] = ot
		}

		return Result{
			Status:   code == 1,
			Code:     code,
			Aciklama: aciklama,
			Person:   b.person,
			Extra:    extra,
			Raw:      respXML,
		}, nil
	}

	// Hiçbir konteyner durum üretmediyse → kayıt bulunamadı
	return Result{
		Status:   false,
		Code:     2,
		Aciklama: "Kayıt bulunamadı",
		Raw:      respXML,
	}, nil
}
