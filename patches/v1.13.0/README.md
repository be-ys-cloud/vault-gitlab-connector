# Vault v1.13.0
* Release date : 2023/03/14
* Authors : Florian Forestier (be ys Cloud), Justine Bachelard (be ys Cloud)


## Changes (server-side)

### New files
* `builtin/credential/gitlab/backend.go` : Add gitlab backend + role parsing
* `builtin/credential/gitlab/backend_test.go` : Tests for gitlab backend
* `builtin/credential/gitlab/cli.go` : CLI options and commands for GitLab backend
* `builtin/credential/gitlab/cmd/gitlab/main.go` : Go start class for gitlab backend builtin plugin
* `builtin/credential/gitlab/path_config.go` : Configuration management for Gitlab backend
* `builtin/credential/gitlab/path_login.go` : Login management for Gitlab backend

### Changed files
* `command/base_predict.go` : Added gitlab as prediction in CLI
* `command/base_predict_test.go` : Added gitlab as prediction in CLI (test file)
* `command/commands.go` : Binding between CLI and GitLab backend
* `go.mod` & `go.sum` : Added `github.com/xanzy/go-gitlab` dependency to facilitate communication between GitLab and Vault
* `helper/builtinplugins/registry.go` : Added GitLab backend binding


## Changes (client-side)

### New files
* `ui/app/adapters/auth-config/gitlab.js` : Create gitLab backend on client side
* `ui/app/models/auth-config/gitlab.js` : Added GitLab configuration class for administration panel
* `ui/app/templates/components/wizard/gitlab-method.hbs` : Added GitLab backend description
* `ui/public/eco/gitlab.svg` : Added gitlab logo for backend configuration (admin panel)

### Changed files
* `ui/app/adapters/cluster.js` : Bind gitlab to other login methods
* `ui/app/components/auth-form.js` : Change the way we manage authentication, because on gitLab's auth form, you will have token, username and password set. You must find the filled one, not only if the field exists.
* `ui/app/helpers/mountable-auth-methods.js` : Add GitLab as a mountable auth method for admin panel
* `ui/app/helpers/supported-auth-backends.js` : Remove all backends that won't be used, and add GitLab. This list will be used on login screen (auth selector)
* `ui/app/helpers/tabs-for-auth-section.js` : Add gitlab tab on admin panel for auth backend configuration
* `ui/app/routes/vault/cluster/settings/auth/configure/section.js` : Added Gitlab endpoints
* `ui/app/templates/auth-form.hbs` : Added gitLab authentication form
* `ui/tests/acceptance/settings/auth/configure/section-test.js` : Added GitLab in acceptance tests
