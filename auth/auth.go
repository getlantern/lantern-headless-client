package auth

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/getlantern/fronted"
	"github.com/getlantern/kindling"
	"github.com/pterm/pterm"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type authClient struct {
	*http.Client
	baseURL string
}

type Client interface {
	SignUp(ctx context.Context, email string, password string) ([]byte, error)
	SignupEmailResendCode(ctx context.Context, data *SignupEmailResendRequest) (bool, error)
	SignupEmailConfirmation(ctx context.Context, data *ConfirmSignupRequest) (bool, error)
	GetSalt(ctx context.Context, email string) (*GetSaltResponse, error)
	LoginPrepare(ctx context.Context, loginData *PrepareRequest) (*PrepareResponse, error)
	Login(ctx context.Context, email string, password string, deviceId string) (*LoginResponse, []byte, error)
	SignOut(ctx context.Context, logoutData *LogoutRequest) (bool, error)
}

// const DefaultAPIURL = "https://api.iantem.io/v1"
const DefaultAPIURL = "https://df.iantem.io/api/v1"

func NewClient(baseURL string, insecure bool, writer io.Writer) Client {
	var httpClient *http.Client
	if insecure {
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			Timeout: 30 * time.Second,
		}
	} else {
		frontedConfig := fronted.NewFronted(
			fronted.WithConfigURL("https://media.githubusercontent.com/media/getlantern/fronted/refs/heads/main/fronted.yaml.gz"),
		)
		transport := kindling.NewKindling(
			kindling.WithLogWriter(writer),
			kindling.WithDomainFronting(frontedConfig),
			kindling.WithProxyless("df.iantem.io"),
		)
		httpClient = transport.NewHTTPClient()

	}
	return &authClient{httpClient, baseURL}
}

func (c *authClient) GetPROTOC(ctx context.Context, path string, params map[string]string, target protoreflect.ProtoMessage) error {
	reqURL, err := url.Parse(fmt.Sprintf("%s%s", c.baseURL, path))
	if err != nil {
		return err
	}
	q := reqURL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	reqURL.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	bo, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	return proto.Unmarshal(bo, target)
}

func (c *authClient) PostPROTOC(ctx context.Context, path string, body protoreflect.ProtoMessage, target protoreflect.ProtoMessage) error {
	bodyBytes, err := proto.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s%s", c.baseURL, path), bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	bo, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	return proto.Unmarshal(bo, target)
}

func (c *authClient) GetSalt(ctx context.Context, email string) (*GetSaltResponse, error) {
	var resp GetSaltResponse
	err := c.GetPROTOC(ctx, "/users/salt", map[string]string{
		"email": email,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *authClient) signUp(ctx context.Context, signupData *SignupRequest) (bool, error) {
	var resp EmptyResponse
	err := c.PostPROTOC(ctx, "/users/signup", signupData, &resp)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *authClient) SignupEmailResendCode(ctx context.Context, data *SignupEmailResendRequest) (bool, error) {
	var resp EmptyResponse
	err := c.PostPROTOC(ctx, "/users/signup/resend/email", data, &resp)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *authClient) SignupEmailConfirmation(ctx context.Context, data *ConfirmSignupRequest) (bool, error) {
	var resp EmptyResponse
	err := c.PostPROTOC(ctx, "/users/signup/complete/email", data, &resp)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *authClient) LoginPrepare(ctx context.Context, loginData *PrepareRequest) (*PrepareResponse, error) {
	var model PrepareResponse
	err := c.PostPROTOC(ctx, "/users/prepare", loginData, &model)
	if err != nil {
		return nil, err
	}
	return &model, nil
}

func (c *authClient) login(ctx context.Context, loginData *LoginRequest) (*LoginResponse, error) {
	pterm.Debug.Printfln("login request is: %v", loginData.String())
	var resp LoginResponse
	err := c.PostPROTOC(ctx, "/users/login", loginData, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *authClient) SignOut(ctx context.Context, logoutData *LogoutRequest) (bool, error) {
	var resp EmptyResponse
	err := c.PostPROTOC(ctx, "/users/logout", logoutData, &resp)
	if err != nil {
		return false, err
	}
	return true, nil
}
