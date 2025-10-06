package kps

type Person string

const (
	PersonTC    Person = "tc_vatandasi"
	PersonYab   Person = "yabanci"
	PersonMavi  Person = "mavi"
	PersonEmpty Person = ""
)

// QueryRequest: sorgu girdisi
type QueryRequest struct {
	TCNo       string `json:"tcno"`
	FirstName  string `json:"firstname"`
	LastName   string `json:"lastname"`
	BirthYear  string `json:"birthyear"`
	BirthMonth string `json:"birthmonth,omitempty"`
	BirthDay   string `json:"birthday,omitempty"`
}

// Result: dışa açık nihai dönüş
// code: 1=Açık, 2=Hatalı/Bulunamadı, 3=Ölüm
type Result struct {
	Status   bool              `json:"status"`             // code == 1 ?
	Code     int               `json:"code"`               // 1, 2, 3
	Aciklama string            `json:"aciklama,omitempty"` // Durum açıklaması
	Person   Person            `json:"person,omitempty"`   // tc_vatandasi | yabanci | mavi
	Extra    map[string]string `json:"extra,omitempty"`    // Ad, Soyad, KimlikNo, Uyruk, DogumTarih, OlumTarih...
	Raw      string            `json:"raw,omitempty"`      // Ham SOAP cevabı (debug)
}
