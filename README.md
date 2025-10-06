# kpsclient

Basit, bağımsız bir Go paketi olarak Nüfus ve Vatandaşlık İşleri (KPS) v2 servislerine sorgu yapmaya yarar.

Bu paket, WS-Trust (STS) isteğini gerçekleştirir, STS tarafından döndürülen SAML tabanlı anahtarla servise HMAC-SHA1 imzalı SOAP isteği gönderir ve gelen yanıtı parse ederek daha kullanışlı bir Go yapısına dönüştürür.

## Öne çıkanlar

- STS (Token Service) ile kimlik doğrulama akışını otomatik olarak işler.
- HMAC-SHA1 ile SOAP mesajlarını imzalar (KPS servisleriyle uyumlu olarak).
- SOAP cevabını parse edip anlamlı `Result` yapısını döndürür.
- Bağımsız, küçük ve kolay kullanılabilir API.

## Kurulum

Go mod ile kullanmak için:

```bash
go get github.com/netinternet/kpsclient
```

veya doğrudan modunuzda:

```go
import kpsclient "github.com/netinternet/kpsclient"
```

## Hızlı Başlangıç

Örnek kullanım `test/main.go` içinde bulunur. Kısaca:

```go
package main

import (
  "context"
  "time"
  kpsclient "github.com/netinternet/kpsclient"
)

func example() {
  client := kpsclient.New("KULLANICI_ADI", "PAROLA", nil)
  req := kpsclient.QueryRequest{
    TCNo:       "99999999999",
    FirstName:  "JOHN",
    LastName:   "DOE",
    BirthYear:  "1990",
    BirthMonth: "01",
    BirthDay:   "01",
  }
  ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
  defer cancel()
  res, err := client.DoQuery(ctx, req)
  if err != nil {
    // hata yönetimi
  }
  // res.Result yapısını kullan
  _ = res
}
```

## API Özeti

- `func New(username, password string, httpClient *http.Client) *Client`
  - Yeni `Client` oluşturur. `httpClient` nil ise 30s timeout'lu varsayılan kullanılır.

- `func (c *Client) DoQuery(ctx context.Context, req QueryRequest) (Result, error)`
  - Verilen sorgu ile STS akışını yürütür, servise imzalı isteği gönderir ve sonucu parse eder.

- `type QueryRequest` (input)
  - `TCNo`, `FirstName`, `LastName`, `BirthYear`, `BirthMonth`, `BirthDay`

- `type Result` (output)
  - `Status` (bool), `Code` (1 başarılı, 2 hatalı/bulunamadı, 3 ölüm), `Aciklama`, `Person` (`tc_vatandasi`, `yabanci`, `mavi`), `Extra` (map), `Raw` (ham SOAP cevabı)

## Konfigürasyon / Ortam Değişkenleri

Paket doğrudan ortam değişkeni okumaz; ancak örnek `test/main.go` dosyası `.env` kullanımı göstermektedir. Gerçek kullanımda `New` fonksiyonuna KPS servislerine kayıtlı kullanıcı adı/parolayı verin.

Not: Bu paket NVI/KPS servislerinin beklediği HMAC-SHA1 imzalama yöntemini kullanır.

Uyarı: KPS servisleri gerçek kimlik doğrulama sağlar; test kredensiyelleri olmadan servis çağrıları hatalı dönebilir veya erişim reddedilebilir.

## Güvenlik ve Notlar

- Paket KPS servisleriyle uyum için HMAC-SHA1 kullanır (STS tarafı gerekliliği). Bu, modern kriptografi tercihleriyle çelişebilir; kullanım alanınıza göre değerlendirin.
- Parolaları ve anahtarları güvenli şekilde saklayın; `.env` dosyaları üretimde uygun değildir.
- Gelen `Raw` alanı hata ayıklama amaçlıdır; gizli bilgi içerebilir — loglarken dikkat edin.
