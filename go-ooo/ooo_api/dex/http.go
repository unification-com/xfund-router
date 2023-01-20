package dex

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

func runQuery(query []byte, url string) ([]byte, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(query))

	if err != nil {
		return nil, err
	}

	httpClient := http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := httpClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			return nil, err
		}

		return body, nil
	} else {
		return nil, errors.New(fmt.Sprintf("non-200 status code: %v", resp.Status))
	}
}
