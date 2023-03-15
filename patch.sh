#!/bin/sh

if [ -z "$1" ]; then
  echo "ERROR : You must define a variable."
  exit 1
fi

if [ ! -d "patches/$1" ]; then
  echo "ERROR : patch folder does not exists for $1."
  exit 1
fi


if [ -z "$GITLAB_TOKEN" ]; then
  if [ -z "$CI_JOB_TOKEN" ]; then
    echo "ERROR : You must define GITLAB_TOKEN environment variable."
    exit 1
  else
    GITLAB_TOKEN=$CI_JOB_TOKEN
  fi
fi

rm -rf vault/ > /dev/null 2>&1

git clone -b $1 https://github.com/hashicorp/vault.git vault
cp -R -f patches/$1/* vault/
echo "SUCCESS : If not error occurred, source have been patched !"
