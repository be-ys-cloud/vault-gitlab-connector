package gitlab

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/sdk/framework"
	gitlab "github.com/xanzy/go-gitlab"
)

func pathConfig(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "config",
		Fields: map[string]*framework.FieldSchema{
			"base_url": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "The Gitlab API endpoint to use.",
			},
			"min_access_level": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "The minimal project access level that users must have",
				Default:     "owner",
			},
			"app_id": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "The OAuth appId",
				Default:     "",
			},
			"app_secret": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "The OAuth appSecret",
				Default:     "",
			},
			"callback_url": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "The Vault OAuth API endpoint to use.",
				Default:     "",
			},
			"ci_token": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "The CI token API to use.",
				Default:     "",
			},
		},

		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.UpdateOperation: b.pathConfigWrite,
			logical.ReadOperation:   b.pathConfigRead,
		},
	}
}

func (b *backend) pathConfigWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	baseURL := data.Get("base_url").(string)
	if len(baseURL) > 0 {
		_, err := url.Parse(baseURL)
		if err != nil {
			return logical.ErrorResponse(fmt.Sprintf("Error parsing given base_url: %s", err)), nil
		}
	}
	minAccessLevel := data.Get("min_access_level").(string)
	appID := data.Get("app_id").(string)
	appSecret := data.Get("app_secret").(string)
	callbackURL := data.Get("callback_url").(string)
	ciToken := data.Get("ci_token").(string)
	if len(callbackURL) > 0 {
		_, err := url.Parse(callbackURL)
		if err != nil {
			return logical.ErrorResponse(fmt.Sprintf("Error parsing given callback_url: %s", err)), nil
		}
	}
	entry, err := logical.StorageEntryJSON("config", config{
		BaseURL:        baseURL,
		MinAccessLevel: minAccessLevel,
		AppID:          appID,
		AppSecret:      appSecret,
		CallbackURL:    callbackURL,
		CIToken:        ciToken,
	})

	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *backend) pathConfigRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	config, err := b.Config(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if config == nil {
		return nil, fmt.Errorf("configuration object not found")
	}

	resp := &logical.Response{
		Data: map[string]interface{}{
			"base_url":         config.BaseURL,
			"min_access_level": config.MinAccessLevel,
			"app_id":           config.AppID,
			"app_secret":       config.AppSecret,
			"callback_url":     config.CallbackURL,
			"ci_token":         config.CIToken,
		},
	}
	return resp, nil
}

// Config returns the configuration for this backend.
func (b *backend) Config(ctx context.Context, s logical.Storage) (*config, error) {
	entry, err := s.Get(ctx, "config")
	if err != nil {
		return nil, err
	}

	var result config
	if entry != nil {
		if err := entry.DecodeJSON(&result); err != nil {
			return nil, errwrap.Wrapf("error reading configuration: {{err}}", err)
		}
	}

	return &result, nil
}

func (b *backend) AccessLevelValue(level string) *gitlab.AccessLevelValue {
	if level == "" {
		return gitlab.AccessLevel(gitlab.OwnerPermission)
	}
	return gitlab.AccessLevel(accessLevelNameToValue[level])
}

func (b *backend) MinAccessLevelValue(level string) []string {
	var accessLevelValues []string

	var start = int(*b.AccessLevelValue(level))
	for k, v := range accessLevelNameToValue {
		if int(v) >= start {
			accessLevelValues = append(accessLevelValues, k)
		}
	}
	return accessLevelValues
}

var accessLevelNameToValue = map[string]gitlab.AccessLevelValue{
	"none":       gitlab.NoPermissions,
	"guest":      gitlab.GuestPermissions,
	"reporter":   gitlab.ReporterPermissions,
	"developer":  gitlab.DeveloperPermissions,
	"maintainer": gitlab.MaintainerPermissions,
	"owner":      gitlab.OwnerPermission,
}

type config struct {
	BaseURL        string `json:"baseURL" structs:"baseURL" mapstructure:"baseURL"`
	MinAccessLevel string `json:"minAccessLevel" structs:"minAccessLevel" mapstructure:"minAccessLevel"`
	AppID          string `json:"appID" structs:"appID" mapstructure:"appID"`
	AppSecret      string `json:"appSecret" structs:"appSecret" mapstructure:"appSecret"`
	CallbackURL    string `json:"callbackURL" structs:"callbackURL" mapstructure:"callbackURL"`
	CIToken        string `json:"ciToken" structs:"ciToken" mapstructure:"ciToken"`
}
