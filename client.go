package kps

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1" // #nosec G505: KPS uçları HMAC-SHA1 istiyor
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/google/uuid"
	dsig "github.com/russellhaering/goxmldsig"
)

const (
	stsURL    = "https://kimlikdogrulama.nvi.gov.tr/Services/Issuer.svc/IWSTrust13"
	queryURL  = "https://kpsv2.nvi.gov.tr/Services/RoutingService.svc"
	soapNS12  = "http://www.w3.org/2003/05/soap-envelope"
	wsuNS     = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"
	wsseNS    = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd"
	wsaNS     = "http://www.w3.org/2005/08/addressing"
	dsigNS    = "http://www.w3.org/2000/09/xmldsig#"
	trustNS   = "http://docs.oasis-open.org/ws-sx/ws-trust/200512"
	methodURI = "http://kps.nvi.gov.tr/2025/08/01/TumKutukDogrulaServis/Sorgula"
)

type Client struct {
	Username   string
	Password   string
	HTTPClient *http.Client
}

// New: basit kurucu. httpClient nil ise makul timeout’lu varsayılan kullanılır.
func New(username, password string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		Username:   username,
		Password:   password,
		HTTPClient: httpClient,
	}
}

// DoQuery: STS → imzalı servis → parse akışını yürütür.
func (c *Client) DoQuery(ctx context.Context, req QueryRequest) (Result, error) {
	arts, rawSTS, err := callSTS(c.Username, c.Password)
	if err != nil {
		return Result{Status: false, Code: 2, Aciklama: fmt.Sprintf("STS hatası: %v", err), Raw: rawSTS}, err
	}

	body := BuildTumKutukBody(req)

	rawSvc, err := c.callSignedService(ctx, arts, body)
	if err != nil {
		return Result{Status: false, Code: 2, Aciklama: fmt.Sprintf("Servis hatası: %v", err), Raw: rawSvc}, err
	}

	parsed, pErr := ParseTumKutukResponse(rawSvc)
	if pErr != nil {
		parsed.Raw = rawSvc
	}
	return parsed, pErr
}

type stsArtifacts struct {
	BinarySecretB64 string // HMAC anahtarı (base64)
	TokenXML        string // RequestedSecurityToken içindeki EncryptedData (inner XML)
	AssertionID     string // KeyIdentifier (SAML Assertion ID)
}

func callSTS(username, password string) (stsArtifacts, string, error) {
	msgID := "urn:uuid:" + uuid.New().String()
	now := time.Now().UTC()
	created := tsISO(now)
	expires := tsISO(now.Add(5 * time.Minute))

	rst := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="%s" xmlns:a="%s" xmlns:wst="%s" xmlns:wsse="%s" xmlns:wsu="%s" xmlns:wsp="http://schemas.xmlsoap.org/ws/2004/09/policy">
  <s:Header>
    <a:MessageID>%s</a:MessageID>
    <a:To>%s</a:To>
    <a:Action>http://docs.oasis-open.org/ws-sx/ws-trust/200512/RST/Issue</a:Action>
    <wsse:Security s:mustUnderstand="1">
      <wsu:Timestamp wsu:Id="_0">
        <wsu:Created>%s</wsu:Created>
        <wsu:Expires>%s</wsu:Expires>
      </wsu:Timestamp>
      <wsse:UsernameToken wsu:Id="Me">
        <wsse:Username>%s</wsse:Username>
        <wsse:Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordText">%s</wsse:Password>
      </wsse:UsernameToken>
    </wsse:Security>
  </s:Header>
  <s:Body>
    <wst:RequestSecurityToken>
      <wst:TokenType>http://docs.oasis-open.org/wss/oasis-wss-saml-token-profile-1.1#SAMLV1.1</wst:TokenType>
      <wst:RequestType>http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue</wst:RequestType>
      <wsp:AppliesTo>
        <a:EndpointReference>
          <a:Address>%s</a:Address>
        </a:EndpointReference>
      </wsp:AppliesTo>
      <wst:KeyType>http://docs.oasis-open.org/ws-sx/ws-trust/200512/SymmetricKey</wst:KeyType>
    </wst:RequestSecurityToken>
  </s:Body>
</s:Envelope>`,
		soapNS12, wsaNS, trustNS, wsseNS, wsuNS,
		msgID, stsURL,
		created, expires,
		xmlEscape(username), xmlEscape(password),
		queryURL,
	)

	req, _ := http.NewRequest("POST", stsURL, bytes.NewReader([]byte(rst)))
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")

	cli := &http.Client{Timeout: 30 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return stsArtifacts{}, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	raw := string(b)
	if resp.StatusCode != http.StatusOK {
		return stsArtifacts{}, raw, fmt.Errorf("sts http %d", resp.StatusCode)
	}
	arts, perr := parseSTSResponse(raw)
	if perr != nil {
		return stsArtifacts{}, raw, perr
	}
	return arts, raw, nil
}

func parseSTSResponse(respXML string) (stsArtifacts, error) {
	dec := xml.NewDecoder(strings.NewReader(respXML))
	var secret, keyID string

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return stsArtifacts{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "BinarySecret":
				var v string
				if err := dec.DecodeElement(&v, &t); err == nil {
					if s := strings.TrimSpace(v); s != "" {
						secret = s
					}
				}
			case "KeyIdentifier":
				var v string
				if err := dec.DecodeElement(&v, &t); err == nil {
					if s := strings.TrimSpace(v); s != "" {
						keyID = s
					}
				}
			}
		}
	}

	tokenInner := extractInnerXMLOfTagAnyNS(respXML, "RequestedSecurityToken")
	if strings.TrimSpace(secret) == "" || strings.TrimSpace(keyID) == "" || strings.TrimSpace(tokenInner) == "" {
		return stsArtifacts{}, fmt.Errorf("STS parse error (secret/keyID/token missing)")
	}
	return stsArtifacts{
		BinarySecretB64: strings.TrimSpace(secret),
		TokenXML:        strings.TrimSpace(tokenInner),
		AssertionID:     strings.TrimSpace(keyID),
	}, nil
}

func (c *Client) callSignedService(ctx context.Context, art stsArtifacts, bodyXML string) (string, error) {
	now := time.Now().UTC()
	created := tsISO(now)
	expires := tsISO(now.Add(5 * time.Minute))
	msgID := "urn:uuid:" + uuid.New().String()

	// (1) Timestamp (Id = _0)
	timestamp := fmt.Sprintf(
		`<wsu:Timestamp xmlns:wsu="%s" wsu:Id="_0"><wsu:Created>%s</wsu:Created><wsu:Expires>%s</wsu:Expires></wsu:Timestamp>`,
		wsuNS, created, expires,
	)

	// (2) Canonicalize & digest
	tsC14N, err := c14nExclusive(timestamp)
	if err != nil {
		return "", fmt.Errorf("c14n timestamp: %w", err)
	}
	tsSha1 := sha1.Sum(tsC14N)
	digestValue := base64.StdEncoding.EncodeToString(tsSha1[:])

	// (3) SignedInfo
	signedInfo := fmt.Sprintf(
		`<dsig:SignedInfo xmlns:dsig="%s">
            <dsig:CanonicalizationMethod Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#"/>
            <dsig:SignatureMethod Algorithm="http://www.w3.org/2000/09/xmldsig#hmac-sha1"/>
            <dsig:Reference URI="#_0">
                <dsig:Transforms>
                    <dsig:Transform Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#"/>
                </dsig:Transforms>
                <dsig:DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"/>
                <dsig:DigestValue>%s</dsig:DigestValue>
            </dsig:Reference>
        </dsig:SignedInfo>`,
		dsigNS, digestValue,
	)

	// (4) HMAC-SHA1(SignatureValue)
	siC14N, err := c14nExclusive(signedInfo)
	if err != nil {
		return "", fmt.Errorf("c14n signedinfo: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(art.BinarySecretB64))
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(siC14N)
	sigB64 := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// (5) Signature bloğu
	signature := fmt.Sprintf(
		`<dsig:Signature xmlns:dsig="%s">
            %s
            <dsig:SignatureValue>%s</dsig:SignatureValue>
            <dsig:KeyInfo>
                <wsse:SecurityTokenReference xmlns:wsse="%s">
                    <wsse:KeyIdentifier ValueType="http://docs.oasis-open.org/wss/oasis-wss-saml-token-profile-1.0#SAMLAssertionID">%s</wsse:KeyIdentifier>
                </wsse:SecurityTokenReference>
            </dsig:KeyInfo>
        </dsig:Signature>`,
		dsigNS, signedInfo, sigB64, wsseNS, xmlEscape(art.AssertionID),
	)

	// (6) Header (WS-Addressing + WS-Security)
	header := fmt.Sprintf(`
        <a:MessageID xmlns:a="%s">%s</a:MessageID>
        <a:To xmlns:a="%s" s:mustUnderstand="1">%s</a:To>
        <a:Action xmlns:a="%s" s:mustUnderstand="1">%s</a:Action>
        <wsse:Security xmlns:wsse="%s" xmlns:wsu="%s" s:mustUnderstand="1">
            %s
            %s
            %s
        </wsse:Security>`,
		wsaNS, msgID, wsaNS, queryURL, wsaNS, methodURI,
		wsseNS, wsuNS,
		timestamp,    // imza bunun üzerinden alındı
		art.TokenXML, // STS’ten gelen EncryptedData inner XML
		signature,
	)

	// (7) Envelope
	envelope := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="%s">
  <s:Header>%s</s:Header>
  <s:Body>%s</s:Body>
</s:Envelope>`, soapNS12, header, bodyXML)

	// (8) POST
	raw, status, err := httpPost(queryURL, "application/soap+xml; charset=utf-8", []byte(envelope))
	if err != nil {
		return raw, err
	}
	if status != http.StatusOK {
		return raw, fmt.Errorf("service http %d", status)
	}
	return raw, nil
}

func httpPost(url, contentType string, body []byte) (string, int, error) {
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)

	cli := &http.Client{Timeout: 30 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp.StatusCode, nil
}
func c14nExclusive(fragment string) ([]byte, error) {
	// etree yalnızca C14N için kullanılıyor
	s := bytes.TrimSpace([]byte(fragment))
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(s); err != nil {
		return nil, err
	}
	el := doc.Root()
	if el == nil {
		return nil, fmt.Errorf("c14nExclusive: empty root element")
	}
	canon := dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	return canon.Canonicalize(el)
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func tsISO(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func extractInnerXMLOfTagAnyNS(xmlStr, local string) string {
	low := strings.ToLower(xmlStr)
	needleOpen := "<" + strings.ToLower(local)
	idx := strings.Index(low, needleOpen)
	if idx < 0 {
		needleOpen = ":" + strings.ToLower(local)
		idx = strings.Index(low, needleOpen)
		if idx < 0 {
			return ""
		}
		idx = strings.LastIndex(low[:idx], "<")
		if idx < 0 {
			return ""
		}
	}
	gt := strings.Index(low[idx:], ">")
	if gt < 0 {
		return ""
	}
	start := idx + gt + 1

	closeTag := "</" + strings.ToLower(local) + ">"
	end := strings.Index(low[start:], closeTag)
	if end < 0 {
		cand := strings.Index(low[start:], strings.ToLower(local)+">")
		if cand >= 0 {
			pre := strings.LastIndex(low[:start+cand+len(local)+1], "</")
			if pre >= 0 {
				end = pre - start
			}
		}
		if end < 0 {
			return ""
		}
	}
	return xmlStr[start : start+end]
}
