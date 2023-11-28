# Vault-Gitlab example : Shell script

In this example, we are logging-in against Vault using a shell script, and exporting three variables to our
environment. `jq.py` helps us to retrieve secrets in JSON payload, but could be discarded using some good ol' grep.

To run this script (note: change your secret path&values in lines 25, 27, 28):
```shell
export VAULT_ADDR=https://<your_vault_server>
. ./login.sh
```