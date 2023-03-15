package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/helper/policyutil"
	"github.com/hashicorp/vault/sdk/helper/strutil"
	"github.com/hashicorp/vault/sdk/logical"
	gitlab "github.com/xanzy/go-gitlab"
)

func pathLoginToken(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: `login`,
		Fields: map[string]*framework.FieldSchema{
			"token": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Gitlab API token",
			},
		},

		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.UpdateOperation:         b.pathLoginByToken,
			logical.AliasLookaheadOperation: b.pathLoginAliasLookahead,
		},
	}
}

func pathLoginUserPass(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: `login/(?P<username>.+)`,
		Fields: map[string]*framework.FieldSchema{
			"username": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Gitlab User name",
			},

			"password": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Password for this user",
			},
		},

		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.UpdateOperation:         b.pathLoginByUserPass,
			logical.AliasLookaheadOperation: b.pathLoginAliasLookahead,
		},
	}
}

func pathOauthLogin(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: `oauth`,
		Fields: map[string]*framework.FieldSchema{
			"code": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Gitlab API code",
			},
			"state": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Gitlab API state",
				Default:     "",
			},
		},

		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.UpdateOperation: b.pathOauthLogin,
			logical.ReadOperation:   b.pathOauthLogin,
		},
	}
}

func pathLoginJob(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: `ci`,
		Fields: map[string]*framework.FieldSchema{
			"project": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Gitlab Project id",
			},

			"job": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Gitlab Job id",
			},

			"commit": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Gitlab Commit id",
			},

			"token": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Gitlab Job token",
			},
		},

		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.UpdateOperation:         b.pathLoginByJob,
			logical.AliasLookaheadOperation: b.pathLoginAliasLookahead,
		},
	}
}

func (b *backend) pathLoginByToken(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	config, err := b.Config(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if verifyResponse, resp, err := b.verifyCredentials(ctx, req, b.TokenClient(config.BaseURL, data.Get("token").(string))); err != nil {
		return nil, err
	} else if resp != nil {
		return resp, nil
	} else {
		return b.pathLoginOk(verifyResponse, map[string]interface{}{
			"token": data.Get("token").(string),
		}), nil
	}
}

func (b *backend) pathLoginByUserPass(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	config, err := b.Config(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	client, err := b.UserPassClient(config.BaseURL, data.Get("username").(string), data.Get("password").(string))
	if err != nil {
		return nil, err
	}

	if verifyResponse, resp, err := b.verifyCredentials(ctx, req, client); err != nil {
		return nil, err
	} else if resp != nil {
		return resp, nil
	} else {
		return b.pathLoginOk(verifyResponse, map[string]interface{}{
			"username": data.Get("username").(string),
			"password": data.Get("password").(string),
		}), nil
	}
}

func (b *backend) pathLoginAliasLookahead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	token, ok := data.GetOk("token")
	if !ok {
		token = ""
	}
	username, ok := data.GetOk("username")
	if !ok {
		username = ""
	}
	password, ok := data.GetOk("password")
	if !ok {
		password = ""
	}

	var client *gitlab.Client
	config, err := b.Config(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if token != "" {
		client = b.TokenClient(config.BaseURL, token.(string))
	} else if username != "" {
		client, err = b.UserPassClient(config.BaseURL, username.(string), password.(string))
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("unknow client type")
	}

	var verifyResp *verifyCredentialsResp
	if verifyResponse, resp, err := b.verifyCredentials(ctx, req, client); err != nil {
		return nil, err
	} else if resp != nil {
		return resp, nil
	} else {
		verifyResp = verifyResponse
	}

	return &logical.Response{
		Auth: &logical.Auth{
			Alias: &logical.Alias{
				Name: verifyResp.Username,
			},
			EntityID: verifyResp.Username,
		},
	}, nil
}

func (b *backend) pathLoginOk(verifyResp *verifyCredentialsResp, internalData map[string]interface{}) *logical.Response {
	resp := &logical.Response{
		Auth: &logical.Auth{
			InternalData: internalData,
			Metadata: map[string]string{
				"username": verifyResp.Username,
			},
			DisplayName: verifyResp.Username,
			LeaseOptions: logical.LeaseOptions{
				Renewable: true,
			},
			Alias: &logical.Alias{
				Name: verifyResp.Username,
			},
			EntityID: verifyResp.Username,
		},
	}

	if verifyResp.IsAdmin {
		if b.Logger().IsDebug() {
			b.Logger().Debug("User " + verifyResp.Username + " is admin")
		}
		resp.Auth.Policies = append(resp.Auth.Policies, "admins")
	}

	aliasNames := append(verifyResp.ProjectNames, verifyResp.GroupNames...)
	aliasNames = strutil.RemoveEmpty(strutil.RemoveDuplicates(aliasNames, true))

	for _, name := range aliasNames {
		resp.Auth.GroupAliases = append(resp.Auth.GroupAliases, &logical.Alias{
			Name: name,
		})
	}

	re := regexp.MustCompile(`[^\w]+`)
	for _, name := range aliasNames {
		resp.Auth.Policies = append(resp.Auth.Policies, re.ReplaceAllString(name, `_`))
	}

	return resp
}

func (b *backend) pathLoginRenew(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	if req.Auth == nil {
		return nil, fmt.Errorf("request auth was nil")
	}

	config, err := b.Config(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	tokenRaw, ok := req.Auth.InternalData["token"]
	if !ok {
		return nil, fmt.Errorf("token created in previous version of Vault cannot be validated properly at renewal time")
	}
	token := tokenRaw.(string)

	var verifyResp *verifyCredentialsResp
	if verifyResponse, resp, err := b.verifyCredentials(ctx, req, b.TokenClient(config.BaseURL, token)); err != nil {
		return nil, err
	} else if resp != nil {
		return resp, nil
	} else {
		verifyResp = verifyResponse
	}
	aliasNames := append(verifyResp.ProjectNames, verifyResp.GroupNames...)
	aliasNames = strutil.RemoveEmpty(strutil.RemoveDuplicates(aliasNames, true))

	if !policyutil.EquivalentPolicies(aliasNames, req.Auth.TokenPolicies) {
		return nil, fmt.Errorf("policies do not match")
	}

	resp := &logical.Response{Auth: req.Auth}
	// Remove old aliases
	resp.Auth.GroupAliases = nil

	for _, name := range aliasNames {
		resp.Auth.GroupAliases = append(resp.Auth.GroupAliases, &logical.Alias{
			Name: name,
		})
	}

	re := regexp.MustCompile(`[^\w]+`)
	for _, name := range aliasNames {
		resp.Auth.Policies = append(resp.Auth.Policies, re.ReplaceAllString(name, `_`))
	}

	return resp, nil
}

func (b *backend) verifyCredentials(ctx context.Context, req *logical.Request, client *gitlab.Client) (*verifyCredentialsResp, *logical.Response, error) {
	config, err := b.Config(ctx, req.Storage)
	if err != nil {
		return nil, nil, err
	}

	// Get the user
	user, _, err := client.Users.CurrentUser()
	if err != nil {
		return nil, nil, err
	}

	username := user.Username
	isAdmin := user.IsAdmin

	var projectNames, groupNames *[]string
	var mux = &sync.Mutex{}
	var wg sync.WaitGroup

	var errorVerify error
	for _, accessLevel := range b.MinAccessLevelValue(config.MinAccessLevel) {
		wg.Add(1)
		go func(accessLevel string) {
			defer wg.Done()
			optProjects := &gitlab.ListProjectsOptions{
				MinAccessLevel: b.AccessLevelValue(accessLevel),
				ListOptions: gitlab.ListOptions{
					PerPage: 100,
				},
			}
			for errorVerify == nil {
				projects, resp, err := client.Projects.ListProjects(optProjects)
				if err != nil {
					errorVerify = err
					return
				}
				t := []string{}
				for _, p := range projects {
					t = append(t, p.PathWithNamespace+"_"+accessLevel)
				}
				mux.Lock()
				if projectNames != nil {
					t = append(*projectNames, t...)
				}
				projectNames = &t
				mux.Unlock()
				if resp.NextPage == 0 {
					break
				}
				optProjects.Page = resp.NextPage
			}
		}(accessLevel)

		wg.Add(1)
		go func(accessLevel string) {
			defer wg.Done()
			optGroups := &gitlab.ListGroupsOptions{
				MinAccessLevel: b.AccessLevelValue(accessLevel),
				ListOptions: gitlab.ListOptions{
					PerPage: 100,
				},
			}
			var allGroups []*gitlab.Group
			for errorVerify == nil {
				groups, resp, err := client.Groups.ListGroups(optGroups)
				if err != nil {
					errorVerify = err
					return
				}
				t := []string{}
				for _, g := range groups {
					t = append(t, g.FullPath+"_"+accessLevel)
				}
				mux.Lock()
				if groupNames != nil {
					t = append(*groupNames, t...)
				}
				groupNames = &t
				mux.Unlock()
				if resp.NextPage == 0 {
					break
				}
				optGroups.Page = resp.NextPage
			}
			for _, g := range allGroups {
				mux.Lock()
				t := append(*groupNames, g.FullPath+"_"+accessLevel)
				groupNames = &t
				mux.Unlock()
			}
		}(accessLevel)
	}
	wg.Wait()

	if errorVerify != nil {
		return nil, nil, errorVerify
	}

	return &verifyCredentialsResp{
		Username:     username,
		ProjectNames: *projectNames,
		GroupNames:   *groupNames,
		IsAdmin:      isAdmin,
	}, nil, nil
}

func (b *backend) pathOauthLogin(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	config, err := b.Config(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if config.AppSecret == "" || config.AppID == "" || config.CallbackURL == "" {
		return nil, fmt.Errorf("config OAuth disabled")
	}

	baseURL, _ := url.Parse(config.BaseURL)
	callbackURL, _ := url.Parse(config.CallbackURL)

	oauth2Conf := &oauth2.Config{
		ClientID:     config.AppID,
		ClientSecret: config.AppSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s://%s/oauth/authorize", baseURL.Scheme, baseURL.Host),
			TokenURL: fmt.Sprintf("%s://%s/oauth/token", baseURL.Scheme, baseURL.Host),
		},
		Scopes:      []string{"api", "read_user"},
		RedirectURL: fmt.Sprintf("%s://%s/v1/%s%s", callbackURL.Scheme, callbackURL.Host, req.MountPoint, req.Path),
	}

	code, _ := data.GetOk("code")

	if code != nil {
		state := data.Get("state")
		err = b.CheckState(state.(string))
		if err != nil {
			return nil, err
		}

		token, err := oauth2Conf.Exchange(ctx, code.(string))
		if err != nil {
			return nil, err
		}
		if verifyResponse, resp, err := b.verifyCredentials(ctx, req, b.TokenClient(config.BaseURL, "OAuth-"+token.AccessToken)); err != nil {
			return nil, err
		} else if resp != nil {
			return resp, nil
		} else {
			response := b.pathLoginOk(verifyResponse, map[string]interface{}{
				"token": token.AccessToken,
			})

			wrappedResponse, err := b.System().ResponseWrapData(ctx, map[string]interface{}{
				"authType": "gitlab",
				"token":    "OAuth-" + token.AccessToken,
			}, time.Second*60, false)
			if err != nil {
				return nil, err
			}
			response.Redirect = "/ui/vault/auth?with=gitlab&wrapped_token=" + wrappedResponse.Token

			return response, nil
		}
	} else {
		state, err := b.State()
		if err != nil {
			return nil, err
		}
		return &logical.Response{
			Redirect: oauth2Conf.AuthCodeURL(state, oauth2.AccessTypeOffline),
		}, nil
	}

}

func (b *backend) pathLoginByJob(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	config, err := b.Config(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if config.CIToken == "" {
		return nil, fmt.Errorf("config CI access disabled")
	}
	client, err := b.JobClient(config.BaseURL, config.CIToken, data.Get("project").(string), data.Get("job").(string), data.Get("commit").(string), data.Get("token").(string))
	if err != nil {
		return nil, err
	}

	if verifyResponse, resp, err := b.verifyCredentials(ctx, req, client); err != nil {
		return nil, err
	} else if resp != nil {
		return resp, nil
	} else {
		return b.pathLoginOk(verifyResponse, map[string]interface{}{
			"project": data.Get("project").(string),
			"job":     data.Get("job").(string),
			"commit":  data.Get("commit").(string),
			"token":   data.Get("token").(string),
		}), nil
	}
}

type verifyCredentialsResp struct {
	Username     string
	ProjectNames []string
	GroupNames   []string
	IsAdmin      bool
}

