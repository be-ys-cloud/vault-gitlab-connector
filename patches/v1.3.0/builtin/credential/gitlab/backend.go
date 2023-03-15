package gitlab

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	mathrand "math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"strings"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/vault/helper/mfa"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
	gitlab "github.com/xanzy/go-gitlab"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Factory of gitlab backend
func Factory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	b := Backend()
	if err := b.Setup(ctx, conf); err != nil {
		return nil, err
	}
	b.CipherKey = make([]byte, 16)
	for i := range b.CipherKey {
		b.CipherKey[i] = letterBytes[mathrand.Intn(len(letterBytes))]
	}
	return b, nil
}

// Backend constructor
func Backend() *backend {
	var b backend
	b.GroupMap = &framework.PolicyMap{
		PathMap: framework.PathMap{
			Name: "groups",
		},
		DefaultKey: "default",
	}

	b.UserMap = &framework.PolicyMap{
		PathMap: framework.PathMap{
			Name: "users",
		},
		DefaultKey: "default",
	}

	allPaths := append(b.UserMap.Paths())
	b.Backend = &framework.Backend{
		Help: backendHelp,

		PathsSpecial: &logical.Paths{
			Root: mfa.MFARootPaths(),
			Unauthenticated: []string{
				"login",
				"login/*",
				"oauth",
				"ci",
			},
		},

		Paths: append([]*framework.Path{
			pathConfig(&b),
		}, append(
			append(
				append(
					append(
						allPaths, mfa.MFAPaths(b.Backend, pathLoginToken(&b))...,
					), mfa.MFAPaths(b.Backend, pathLoginUserPass(&b))...,
				), mfa.MFAPaths(b.Backend, pathOauthLogin(&b))...,
			), mfa.MFAPaths(b.Backend, pathLoginJob(&b))...,
		)...,
		),
		AuthRenew:   b.pathLoginRenew,
		BackendType: logical.TypeCredential,
	}

	return &b
}

type backend struct {
	*framework.Backend

	GroupMap *framework.PolicyMap

	UserMap *framework.PolicyMap

	CipherKey []byte
}

// Client returns the Gitlab client to communicate to Gitlab via the
// configured settings.

func (b *backend) TokenClient(token string) *gitlab.Client {
	tc := cleanhttp.DefaultClient()
	if strings.HasPrefix(token, "OAuth-") {
		return gitlab.NewOAuthClient(tc, strings.TrimPrefix(token, "OAuth-"))
	}
	return gitlab.NewClient(tc, token)
}

func (b *backend) UserPassClient(baseURL, username, password string) (*gitlab.Client, error) {
	tc := cleanhttp.DefaultClient()
	client, err := gitlab.NewBasicAuthClient(tc, baseURL, username, password)
	if err != nil {
		return nil, err
	}
	return client, nil
}

type sudoRoundTripper struct {
	http.RoundTripper
	wrapped http.RoundTripper
	user    string
}

func (s *sudoRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("SUDO", s.user)
	return s.wrapped.RoundTrip(req)
}

func (b *backend) JobClient(baseURL, CIToken, project, job, commit, token string) (*gitlab.Client, error) {
	tc := cleanhttp.DefaultClient()
	client := gitlab.NewClient(tc, CIToken)
	client.SetBaseURL(baseURL)
	jobID, err := strconv.Atoi(job)
	if err != nil {
		return nil, err
	}
	j, _, err := client.Jobs.GetJob(project, jobID)
	if err != nil {
		return nil, err
	}
	if j.Status != "running" || j.Commit.ID != commit {
		return nil, fmt.Errorf("Invalid job arguments : %s %s %s %s", project, job, commit, token)
	}
	if err := testJobToken(baseURL, project, jobID, token); err != nil {
		return nil, err
	}
	tc.Transport = &sudoRoundTripper{wrapped: tc.Transport, user: strconv.Itoa(j.User.ID)}
	return client, nil
}

func testJobToken(baseURL, project string, jobID int, token string) error {
	u, err := url.Parse(fmt.Sprintf("%s/projects/%s/jobs/%d/artifacts?job_token=%s", baseURL, project, jobID, token))
	if err != nil {
		return err
	}
	resp, err := cleanhttp.DefaultClient().Do(&http.Request{
		Method:     "GET",
		URL:        u,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Host:       u.Host,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode != http.StatusNotFound {
		return errors.New(resp.Status)
	}
	return nil
}

func (b *backend) State() (encmess string, err error) {
	plainText := []byte(strconv.FormatInt(time.Now().UnixNano(), 10))

	block, err := aes.NewCipher(b.CipherKey)
	if err != nil {
		return
	}

	//IV needs to be unique, but doesn't have to be secure.
	//It's common to put it at the beginning of the ciphertext.
	cipherText := make([]byte, aes.BlockSize+len(plainText))
	iv := cipherText[:aes.BlockSize]
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(cipherText[aes.BlockSize:], plainText)

	//returns to base64 encoded string
	encmess = base64.URLEncoding.EncodeToString(cipherText)
	return
}

func (b *backend) CheckState(securemess string) (err error) {
	now := time.Now().UnixNano()
	cipherText, err := base64.URLEncoding.DecodeString(securemess)
	if err != nil {
		return
	}

	block, err := aes.NewCipher(b.CipherKey)
	if err != nil {
		return
	}

	if len(cipherText) < aes.BlockSize {
		err = errors.New("Illegal State")
		return
	}

	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(cipherText, cipherText)

	decodedmess, err := strconv.ParseInt(string(cipherText), 10, 64)
	if err == nil && (decodedmess > now || now-decodedmess > 60*int64(time.Second)) {
		err = errors.New("Illegal State")
	}
	return
}

const backendHelp = `
The Gitlab credential provider allows authentication via Gitlab.

Users provide a personal access token to log in, and the credential
provider maps the user to a set of Vault policies according to the groups he is part of.
After enabling the credential provider, use the "config" route to
configure it.
`
