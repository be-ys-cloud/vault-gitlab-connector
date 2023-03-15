package gitlab

import (
"context"
"os"
"strings"
"testing"

logicaltest "github.com/hashicorp/vault/helper/testhelpers/logical"
"github.com/hashicorp/vault/sdk/logical"
)

func TestBackend_Config(t *testing.T) {
	b, err := Factory(context.Background(), &logical.BackendConfig{
		Logger: nil,
	})
	if err != nil {
		t.Fatalf("Unable to create backend: %s", err)
	}

	loginData := map[string]interface{}{
		// This token has to be replaced with a working token for the test to work.
		"token": os.Getenv("GITLAB_TOKEN"),
	}
	configData := map[string]interface{}{
		"group": os.Getenv("GITLAB_GROUP"),
	}

	logicaltest.Test(t, logicaltest.TestCase{
		PreCheck:       func() { testAccPreCheck(t) },
		LogicalBackend: b,
		Steps: []logicaltest.TestStep{
			testConfigWrite(t, loginData),
			testLoginWrite(t, configData, false),
		},
	})
}

func testLoginWrite(t *testing.T, d map[string]interface{}, expectFail bool) logicaltest.TestStep {
	return logicaltest.TestStep{
		Operation: logical.UpdateOperation,
		Path:      "login",
		ErrorOk:   true,
		Data:      d,
		Check: func(resp *logical.Response) error {
			if resp.IsError() && expectFail {
				return nil
			}
			return nil
		},
	}
}

func testConfigWrite(t *testing.T, d map[string]interface{}) logicaltest.TestStep {
	return logicaltest.TestStep{
		Operation: logical.UpdateOperation,
		Path:      "config",
		Data:      d,
	}
}

func TestBackend_basic(t *testing.T) {
	b, err := Factory(context.Background(), &logical.BackendConfig{
		Logger: nil,
	})
	if err != nil {
		t.Fatalf("Unable to create backend: %s", err)
	}

	logicaltest.Test(t, logicaltest.TestCase{
		PreCheck:       func() { testAccPreCheck(t) },
		LogicalBackend: b,
		Steps: []logicaltest.TestStep{
			testAccStepConfig(t, false),
			testAccMap(t, "default", "fakepol"),
			testAccMap(t, "oWnErs", "fakepol"),
			testAccLogin(t, []string{"default", "fakepol"}),
			testAccStepConfig(t, true),
			testAccMap(t, "default", "fakepol"),
			testAccMap(t, "oWnErs", "fakepol"),
			testAccLogin(t, []string{"default", "fakepol"}),
			testAccStepConfigWithBaseURL(t),
			testAccMap(t, "default", "fakepol"),
			testAccMap(t, "oWnErs", "fakepol"),
			testAccLogin(t, []string{"default", "fakepol"}),
			testAccMap(t, "default", "fakepol"),
			testAccStepConfig(t, true),
			mapUserToPolicy(t, os.Getenv("GITLAB_USER"), "userpolicy"),
			testAccLogin(t, []string{"default", "fakepol", "userpolicy"}),
		},
	})
}

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("GITLAB_TOKEN"); v == "" {
		t.Skip("GITLAB_TOKEN must be set for acceptance tests")
	}

	if v := os.Getenv("GITLAB_GROUP"); v == "" {
		t.Skip("GITLAB_GROUP must be set for acceptance tests")
	}

	if v := os.Getenv("GITLAB_BASEURL"); v == "" {
		t.Skip("GITLAB_BASEURL must be set for acceptance tests (use 'https://gitlab.com/api/v4/' if you don't know what you're doing)")
	}
}

func testAccStepConfig(t *testing.T, upper bool) logicaltest.TestStep {
	ts := logicaltest.TestStep{
		Operation: logical.UpdateOperation,
		Path:      "config",
		Data: map[string]interface{}{
			"organization": os.Getenv("GITLAB_GROUP"),
		},
	}
	if upper {
		ts.Data["organization"] = strings.ToUpper(os.Getenv("GITLAB_GROUP"))
	}
	return ts
}

func testAccStepConfigWithBaseURL(t *testing.T) logicaltest.TestStep {
	return logicaltest.TestStep{
		Operation: logical.UpdateOperation,
		Path:      "config",
		Data: map[string]interface{}{
			"organization": os.Getenv("GITLAB_GROUP"),
			"base_url":     os.Getenv("GITLAB_BASEURL"),
		},
	}
}

func testAccMap(t *testing.T, k string, v string) logicaltest.TestStep {
	return logicaltest.TestStep{
		Operation: logical.UpdateOperation,
		Path:      "map/teams/" + k,
		Data: map[string]interface{}{
			"value": v,
		},
	}
}

func mapUserToPolicy(t *testing.T, k string, v string) logicaltest.TestStep {
	return logicaltest.TestStep{
		Operation: logical.UpdateOperation,
		Path:      "map/users/" + k,
		Data: map[string]interface{}{
			"value": v,
		},
	}
}

func testAccLogin(t *testing.T, policies []string) logicaltest.TestStep {
	return logicaltest.TestStep{
		Operation: logical.UpdateOperation,
		Path:      "login",
		Data: map[string]interface{}{
			"token": os.Getenv("GITLAB_TOKEN"),
		},
		Unauthenticated: true,

		Check: logicaltest.TestCheckAuth(policies),
	}
}
