package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"
)

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type deviceTokenResponse struct {
	Status string `json:"status"`
	APIKey string `json:"api_key"`
	Error  string `json:"error"`
}

type meResponse struct {
	User struct {
		PublicID    string `json:"public_id"`
		Email       string `json:"email"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	} `json:"user"`
	Account struct {
		PublicID  string `json:"public_id"`
		Namespace string `json:"namespace"`
	} `json:"account"`
}

func runLogin() {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	token := fs.String("token", "", "API key (skip browser flow)")
	fs.Parse(os.Args[2:])

	serverURL := resolveServerURL()
	apiURL := serverURL + "/api"

	if *token != "" {
		loginWithToken(serverURL, apiURL, *token)
		return
	}

	loginWithDeviceFlow(serverURL, apiURL)
}

func loginWithToken(serverURL, apiURL, token string) {
	// Verify the token works
	user, err := verifyToken(apiURL, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid API key: %v\n", err)
		os.Exit(1)
	}

	if err := saveCredentials(serverURL, token); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save token: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Logged in as %s (%s)\n", user.User.Username, user.User.Email)
}

func loginWithDeviceFlow(serverURL, apiURL string) {
	// Step 1: Request device code
	resp, err := http.Post(apiURL+"/v1/device/code", "application/json", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot reach %s: %v\n", apiURL, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "Error: server returned %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var dcResp deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcResp); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid response: %v\n", err)
		os.Exit(1)
	}

	// Step 2: Show code and open browser
	fmt.Println()
	fmt.Println("  Your code: " + dcResp.UserCode)
	fmt.Println()
	fmt.Println("  Opening browser to " + dcResp.VerificationURL)
	fmt.Println("  Enter the code above to authorize this device.")
	fmt.Println()

	openBrowser(dcResp.VerificationURL)

	// Step 3: Poll for approval
	fmt.Print("  Waiting for approval...")

	interval := time.Duration(dcResp.Interval) * time.Second
	deadline := time.Now().Add(time.Duration(dcResp.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		result, err := pollDeviceToken(apiURL, dcResp.DeviceCode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
			os.Exit(1)
		}

		switch result.Status {
		case "approved":
			fmt.Println(" approved!")
			fmt.Println()

			if err := saveCredentials(serverURL, result.APIKey); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to save token: %v\n", err)
				os.Exit(1)
			}

			user, err := verifyToken(apiURL, result.APIKey)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: logged in but could not verify: %v\n", err)
				return
			}

			fmt.Printf("  Logged in as %s (%s)\n", user.User.Username, user.User.Email)
			return

		case "expired":
			fmt.Println(" expired.")
			fmt.Fprintln(os.Stderr, "Error: device code expired. Please try again.")
			os.Exit(1)

		case "pending":
			fmt.Print(".")
		}
	}

	fmt.Println(" timed out.")
	fmt.Fprintln(os.Stderr, "Error: timed out waiting for approval. Please try again.")
	os.Exit(1)
}

func pollDeviceToken(apiURL, deviceCode string) (*deviceTokenResponse, error) {
	resp, err := http.PostForm(apiURL+"/v1/device/token", url.Values{
		"device_code": {deviceCode},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot reach server: %w", err)
	}
	defer resp.Body.Close()

	var result deviceTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	// 428 = pending, 410 = expired, 200 = approved, 404 = not found
	switch resp.StatusCode {
	case 200:
		result.Status = "approved"
	case 428:
		result.Status = "pending"
	case 410:
		result.Status = "expired"
	default:
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, result.Error)
	}

	return &result, nil
}

func verifyToken(apiURL, token string) (*meResponse, error) {
	req, err := http.NewRequest("GET", apiURL+"/v1/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var me meResponse
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	return &me, nil
}

func saveCredentials(serverURL, token string) error {
	cfg, err := readGlobalConfig()
	if err != nil {
		return err
	}
	cfg.setServerCredentials(serverURL, token)
	return writeGlobalConfig(cfg)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	cmd.Start()
}
