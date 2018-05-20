#!/bin/sh

if [ ! -f "github-top" ]
then
  curl -L -O https://github.com/lauripiispanen/most-active-github-users-counter/releases/download/v1.7/github-top
  chmod u+x github-top
fi

./github-top --token "$1" --preset $2 --output yaml --file "$2.yml"

echo "page: $2.html\ntitle: $3" | cat - "$2.yml" > "_data/locations/$2.yml"
echo "---\ntype: location\nlocation: $2\n---" > "$2.md"
