package scanner

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reEmail  = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	reAPIKey = regexp.MustCompile(`(sk-|AKIA|Bearer |ghp_)[a-zA-Z0-9]{20,}`)
	reSep    = regexp.MustCompile(`[^a-z0-9]+`)
)

var medicalTerms = []string{
	"patient", "diagnosis", "prescription", "clinical", "phi",
}

var personalNameNeedles = []string{
	"fullname", "firstname", "lastname", "middlename", "maidenname",
	"givenname", "surname", "realname", "displayname", "nickname",
	"username", "personname", "customername", "clientname", "patientname",
	"membername", "employeename", "contactname", "sendername", "recipientname",
	"authorname", "accountholder",
	"emailaddress", "ssn", "socialsecurity", "passport", "driverlicense",
	"creditcard", "cardnumber", "dateofbirth", "birthdate", "phonenumber",
	"homeaddress", "billingaddress", "shippingaddress", "mailingaddress",
	"streetaddress", "postaladdress",
}

var personalNameExact = map[string]bool{
	"name": true, "email": true, "author": true, "user": true,
	"phone": true, "dob": true, "address": true,
}

var secretNeedles = []string{
	"password", "passwd", "secret", "apikey", "accesskey", "privatekey",
	"clientsecret", "authtoken", "accesstoken", "refreshtoken", "bearertoken",
	"sessiontoken", "credential",
}

type piiSignal string

const (
	sigEmail   piiSignal = "email"
	sigAPIKey  piiSignal = "api_key"
	sigMedical piiSignal = "medical"
	sigName    piiSignal = "personal_name"
)

func normalizeKey(k string) string {
	var b strings.Builder
	runes := []rune(k)
	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' && runes[i-1] >= 'a' && runes[i-1] <= 'z' {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return reSep.ReplaceAllString(strings.ToLower(b.String()), "")
}

func classifyFieldName(key string) piiSignal {
	norm := normalizeKey(key)
	if norm == "" {
		return ""
	}
	if personalNameExact[norm] {
		return sigName
	}
	for _, n := range secretNeedles {
		if strings.Contains(norm, n) {
			return sigAPIKey
		}
	}
	for _, n := range personalNameNeedles {
		if strings.Contains(norm, n) {
			return sigName
		}
	}
	return ""
}

// scanRecords checks Milvus entity records for PII signals in field names and values.
func scanRecords(records []map[string]interface{}) []string {
	found := make(map[piiSignal]bool)
	for _, rec := range records {
		for k, v := range rec {
			if sig := classifyFieldName(k); sig != "" {
				found[sig] = true
			}
			val := fmt.Sprintf("%v", v)
			if reEmail.MatchString(val) {
				found[sigEmail] = true
			}
			if reAPIKey.MatchString(val) {
				found[sigAPIKey] = true
			}
			lower := strings.ToLower(val)
			for _, term := range medicalTerms {
				if strings.Contains(lower, term) {
					found[sigMedical] = true
					break
				}
			}
		}
	}
	var out []string
	for sig := range found {
		out = append(out, string(sig))
	}
	return out
}

// recordKeys returns the unique set of field names across sampled records.
func recordKeys(records []map[string]interface{}) []string {
	seen := make(map[string]bool)
	for _, r := range records {
		for k := range r {
			seen[k] = true
		}
	}
	var keys []string
	for k := range seen {
		keys = append(keys, k)
	}
	return keys
}
