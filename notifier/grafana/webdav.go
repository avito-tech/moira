package grafana

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"time"
)

const chars = "abcdefghijklmnopqrstuvwxyz0123456789"

func randomString(n int) string {
	result := make([]byte, n)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

type WebdavUploader struct {
	url       string
	username  string
	password  string
	publicURL string
}

func (u *WebdavUploader) Upload(imgData []byte) (string, error) {
	parsedURL, _ := url.Parse(u.url)
	filename := "moira2_" + randomString(20) + ".png"
	parsedURL.Path = path.Join(parsedURL.Path, filename)

	req, err := http.NewRequest("PUT", parsedURL.String(), bytes.NewReader(imgData))

	if u.username != "" {
		req.SetBasicAuth(u.username, u.password)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		body, _ := ioutil.ReadAll(res.Body)
		return "", fmt.Errorf("failed to upload image. Returned statuscode %v body %s", res.StatusCode, body)
	}

	if u.publicURL != "" {
		publicURL, _ := url.Parse(u.publicURL)
		publicURL.Path = path.Join(publicURL.Path, filename)
		return publicURL.String(), nil
	}

	return parsedURL.String(), nil
}

func NewWebdavImageUploader(url, username, password, publicURL string) *WebdavUploader {
	return &WebdavUploader{
		url:       url,
		username:  username,
		password:  password,
		publicURL: publicURL,
	}
}
