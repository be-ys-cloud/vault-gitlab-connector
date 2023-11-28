#!/usr/bin/env bash

### Part 1 - Login to Vault and retrieve a Vault Token, using CI_JOB_TOKEN or GITLAB_TOKEN

# Login to Vault using multiple methods (CI auto-login or Gitlab token)
if [ -n "$CI_COMMIT_SHA" ]; then
  echo "CI detected. Retrieving vault token for this job ..."
  VAULT_TOKEN=$(curl -s -X POST -k --data "{\"token\":\"$CI_JOB_TOKEN\"}" $VAULT_ADDR/v1/auth/gitlab/ci | python3 /tools/jq.py "auth/client_token")
else #Login through Gitlab Token
  if [ -z "$GITLAB_TOKEN" ]; then
    echo "GITLAB_TOKEN is missing. Please enter your GitLab token: "
    read -sr GITLAB_TOKEN_INPUT
    export GITLAB_TOKEN=$GITLAB_TOKEN_INPUT
  fi
  echo "Gitlab Token detected. Retrieving vault token for this gitlab token ..."
  VAULT_TOKEN=$(curl -s -X POST -k --data "{\"token\":\"$GITLAB_TOKEN\"}" "$VAULT_ADDR/v1/auth/gitlab/login" | python3 /tools/jq.py "auth/client_token")
fi

export VAULT_TOKEN=$VAULT_TOKEN


### Part 2 - Retrieve a secret and export content

# Retrieve whole secret
VAULT_SECRET=$(curl -s -X GET -k --header "X-Vault-Token: $VAULT_TOKEN" "$VAULT_ADDR/v1/gitlab/data/path/to/my/secret")

# Export variables
export USER=$(echo "$VAULT_SECRET" | python3 /tools/jq.py "data/data/user")
export PASS=$(echo "$VAULT_SECRET" | python3 /tools/jq.py "data/data/pass")
