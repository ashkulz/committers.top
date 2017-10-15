#!/bin/sh

now="$(date +'%d-%m-%Y')"

git checkout -B "$now"

./generate.sh $1 finland Finland
./generate.sh $1 germany Germany
./generate.sh $1 japan Japan
./generate.sh $1 netherlands Netherlands
./generate.sh $1 norway Norway
./generate.sh $1 russia Russia
./generate.sh $1 sweden Sweden
./generate.sh $1 ukraine Ukraine

git add _data
git commit -m "updated data"
git push origin "$now"
curl -v -X POST -H "Authorization: token $1" -H "Content-type: application/vnd.github.v3+json" -d "{ \"title\": \"Data update $now\", \"head\": \"$now\", \"base\": \"master\"}" https://api.github.com/repos/lauripiispanen/github-top/pulls

