#!/bin/sh

if [ -z "$1" ]; then
  echo "ERROR : You must define a variable."
  exit 1
fi

if [ ! -f "patches/$1.patch" ]; then
  echo "ERROR : patch does not exists for $1."
  exit 1
fi


rm -rf vault/ > /dev/null 2>&1

git clone -b $1 https://github.com/hashicorp/vault.git vault
cd ./vault
git apply ../patches/$1.patch
echo "SUCCESS : If not error occurred, source have been patched !"
