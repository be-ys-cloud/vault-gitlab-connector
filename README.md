# Vault Patches

Project which takes [HashiCorp Vault](https://github.com/hashicorp/vault/) sources, and add Gitlab interconnexion on it.

> ***Warning*** : Before v1.15.6, policies had different name format.  
> Before migration to v1.15.6, don't forget to duplicate all your policies with the new formatted names.  
> After migration, you'll be able to remove old policies.

> **Please note that the GitLab connector is only available with a full-Vault build starting 1.18.0.

## Building the image

### How it works

* The only thing you have to do is to run the `patch.sh` script provided, with the specified version you want (
  eg: `patch.sh v1.15.1`)
    * The script starts by cloning Vault sources from the specified version to the `vault/` directory
    * Then, the git diff in `patches/` folder matching the specified version is applied
* Patched sources now resides in `vault/` folder. You can now edit or build the Vault project with our modifications!
* For convenience, you will also find a ready-to-go docker file in `docker/` folder. You can build the project
  using `docker build -t customized_vault:latest -f docker/Dockerfile .`

### Run it

**This command is only designed to test your Vault build. DO NOT use this configuration on production!**

`docker run --rm -p 8200:8200 -e VAULT_API_ADDR="http://localhost" --cap-add=IPC_LOCK  customized_vault:latest`

### How to make a new patch ?

While updating Vault to a new version, we strongly suggest you to start by copying the previous version folder patches
and use it as a base.

Vault APIs are quite stable, so you (theoretically) will not spend a lot of times on migration.

To patch and ignore errors, you can run `git apply --reject --whitespace=fix ../patches/<DESIRED_VERSION>.patch`. Then
fix all files that were rejected. Run a `git add . && git commit -m "new version"` in `vault` directory, then
a `git diff HEAD~1 > ../patches/v<DESIRED_VERSION>.patch` to save the full diff.

Starting v1.18.0, you can use the `patch.sh` command by adding a `--dev` argument at the end of the command. For
example: `./patch.sh v1.18.0 --dev`.

_____________

## Using the connector (as an administrator)

*For the upcoming chapter, we consider that you have a customized Vault running. For a configuration breakdown & best
practices, please refer to
[the official HashiCorp Vault documentation](https://developer.hashicorp.com/vault/docs?ajs_aid=fa4225f4-6af7-4d18-b05a-4880f7449f41&product_intent=vault).*

### GitLab configuration

* First of all, you will need to create a GitLab token that can impersonate users. We need it in order to transform
  our `CI_JOB_TOKEN` into user rights. As `CI_JOB_TOKEN` could not access directly to the GitLab APIs we are interested
  in, we have to create an impersonation token, that is immediately removed when authentication succeed.
    * Log-in to GitLab with a user that is able to impersonate (we suggest to use `root` or another "service account",
      as this account is not a personal account and thus will never be disabled) ;
    * Go to https://<your_gitlab_instance>/-/profile/personal_access_tokens, and generate a Personal Access Token with (
      at least) the following rights:
        * `api`
        * `read_api`
        * `read_user`
        * `sudo`
    * **Caution: Starting Gitlab 16.0, Personal Access Token must have an expiration date. Please, be aware of the
      expiration date of the token, and do the appropriate action to generate a new one before it expires.**
    * Keep the generated PAT in a safe place, we'll use it later in Vault configuration
* Then, we have to create an OAuth2 application that will enable the possibility for users to log-in seamlessly to Vault
  using GitLab callbacks.
    * Go to https://<your_gitlab_instance>/admin/applications, and create a new one
        * Name: what you want
            * eg: Vault
        * Redirect URI: The URI of your (yet to configure) Vault Server, with `/v1/auth/gitlab/oauth` as final path
            * eg: `https://my_vault_server.local/v1/auth/gitlab/oauth`
        * You can define "Trusted" parameter as you want. If trusted is set to true, people will not have a prompt from
          GitLab to confirm that user wants to connect to GitLab.
        * Set the "confidential" parameter to "true"
        * Scopes: you must, at least, give the following scopes to the application:
            * `read_api`
            * `read_user`
            * `openid`
            * `profile`
    * Keep the created Application ID and Secret in a safe place.

Congratulations, GitLab is now fully configured! Let's move on to the Vault side.

### Vault

* Log-in with the Vault root Token.
* Go to Access -> Authentication Method, and add a new method. Select the "GitLab" method.
    * Leave all variables by default, and click on "Enable method".
* Then, go to the freshly created authentication method, click on "Configure", and then on "Gitlab Options". You have a
  few fields to fill:
    * Base URL : The address of your GitLab instance.
        * eg: `https://my_gitlab_server.local`.
    * Minimal Access Level: This is the minimum access that will be used to match user rights.
        * For example, if this value is set to "maintainer", only "maintainer" and "owner" roles will be parsed.
        * This variable could help when you have to deal with users who have a lot of projects to speed up their
          connection flow.
        * By default, you can safely keep the option to `guest`, or `reporter`.
    * OAuth Application ID & OAuth Application Secret: put in the previously generated values from GitLab.
    * OAuth callback URL: It should be the root address of your Vault instance.
        * eg: `https://my_vault_server.local`.
    * CI token: put in the previously generated PAT from GitLab.

Vault is now configured, you should be able to log in to Vault using your GitLab credentials now!

### Setting-up policies

When logging-in a user from GitLab, Vault will retrieve the list of granted gitlab projects and groups for the user.
This enables us to use these information as an automated policy loader.

Basically, Vault will replace all non-alphanumeric characters from the group/project path to underscores (`_`), and
concatenate the user role after it. A few examples to fully understand it:

* A user have access to `group/project1` with role `maintainer`
    * The matching policy will be named `group:project1:maintainer`.
* A user have access to `group-one/` with role `owner`, and `group-two/very/long/pa-th/project` with role `reporter`
    * The matching policies will be named `group_one:owner` and `group_two:very:long:pa_th:project:reporter`

Every matching Vault policy will be loaded to the user token.

_____________

## Using the connector (as a user)

### Connecting to Vault UI

* Go to your Vault instance, and select `GitLab` authentication method. Multiple options are available:
    * The most simple way: log-in using OAuth2: it will redirect you to your GitLab instance to prove your identity, and
      log you back into Vault.
    * You can also provide a Gitlab Personal Access Token, or a Gitlab username & password.

### Connecting to Vault using APIs

To log in to Vault using your Gitlab profile, you can use :

* A GitLab Personal Access Token
* Or a temporary and automatically
  generated `CI_JOB_TOKEN` ([see GitLab documentation about it](https://docs.gitlab.com/ee/ci/jobs/ci_job_token.html))

In both case, you will have to do a request to your Vault server to generate a Vault token from your Gitlab token. Then,
you will be able to use your Vault token for all subsequent calls to Vault.

#### GitLab PAT (Personal Access Token)

Using a GitLab PAT, you must send a POST request to `https://<your_vault_instance>/v1/auth/gitlab/login`, with the
following payload:

```json
{
  "token": "YOUR_GITLAB_PAT_HERE"
}
```

If authentication succeed, you will receive a `201 CREATED` response from Vault server, and you will be able to find
your token in the returned JSON payload, in `auth/client_token`.

#### GitLab CI_JOB_TOKEN

Using a GitLab PAT, you must send a POST request to `https://<your_vault_instance>/v1/auth/gitlab/ci`, with the
following payload:

```json
{
  "token": "YOUR_GITLAB_CI_JOB_TOKEN_HERE"
}
```

If authentication succeed, you will receive a `201 CREATED` response from Vault server, and you will be able to find
your token in the returned JSON payload, in `auth/client_token`.

#### Examples

You will be able to find some examples of scripts using these connection methods in `examples` folder.

_____________

## Contributing & licence

All contributions are welcome, as you agree with the licence defined by this project. Feel free to open a PR, we will
have a close look on it (and we thank you in advance for your participation)!

This project is licenced under MIT. Please remember that HashiCorp Vault is under BSL, therefore our licence **only
applies to patch files**. You still have to reach HashiCorp team if you want to sell this software as a service. 
