package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/viper"
)

type APIClient struct {
	BaseURL string
	Client  *http.Client
}

func NewAPIClient() *APIClient {
	baseURL := viper.GetString("api.url")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return &APIClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *APIClient) Get(path string, target interface{}) error {
	resp, err := c.Client.Get(c.BaseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *APIClient) Post(path string, body interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		// Serialize body if needed, or pass nil
		// For simple commands we often don't need a body
		// Simplified for this example
	}

	req, err := http.NewRequest("POST", c.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error: %d", resp.StatusCode)
	}

	return nil
}
