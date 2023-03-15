# Vault Patches

Project who takes Vault sources, and add Gitlab interconnexion on it.

Documentation is still WIP.

## How it works
* Vault clone, and patches injection are done through the `patch.sh` script. You must provide him a version (eg: `patch.sh v1.13.0`)
* You must add the changed/created files into the `patches` folder. Create a subfolder with the version name you are patching.
* The `patch.sh` script takes Vault source-code, and checkout to the version you specified. Then, it copies files in place of the checked ones.
* You are now able to dev or build using the patched sources.

## Update strategy
While updating Vault to a new version, we strongly suggest you to start by copying the previous version folder patches and use it as a base. 
Vault APIs are quite stable, so you (theoretically) will not spend a lot of times on migration...
