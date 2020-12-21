// nolint
package dto

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"go.avito.ru/DO/moira"
)

var ipMask *net.IPNet

const webhook string = "webhook"
const avitoSuffix string = ".avito.ru"

func init() {
	_, ipMask, _ = net.ParseCIDR("10.0.0.0/8")
}

type ContactList struct {
	List []*moira.ContactData `json:"list"`
}

func (*ContactList) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type Contact struct {
	Type          string `json:"type"`
	Value         string `json:"value"`
	FallbackValue string `json:"fallback_value,omitempty"`
	ID            string `json:"id,omitempty"`
	User          string `json:"user,omitempty"`
}

func (*Contact) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (contact *Contact) Bind(r *http.Request) error {
	if contact.Type == "" {
		return fmt.Errorf("Contact type can not be empty")
	}
	if contact.Value == "" {
		return fmt.Errorf("Contact value of type %s can not be empty", contact.Type)
	}
	if contact.Type == webhook {
		return validateWebhook(contact.Value)
	}
	return nil
}

func validateWebhook(u string) error {
	u = strings.ToLower(u)
	parsed, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("unable to parse url")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("incorrect scheme %s", parsed.Scheme)
	}

	hostAndPort := strings.SplitN(parsed.Host, ":", 2)
	host := hostAndPort[0]
	if len(hostAndPort) == 2 {
		port := hostAndPort[1]
		portNum, err := strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("incorrect port %s", port)
		}
		if portNum < 1 || portNum > 65535 {
			return fmt.Errorf("incorrect port %s", port)
		}
	}

	if strings.HasSuffix(host, avitoSuffix) {
		return nil
	} else {
		ip := net.ParseIP(host)
		if ip != nil && ipMask.Contains(ip) {
			return nil
		}
	}

	return fmt.Errorf("incorrect host %s", parsed.Host)

}
