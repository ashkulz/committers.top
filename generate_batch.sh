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

sleep 3600

./generate.sh $1 estonia Estonia
./generate.sh $1 denmark Denmark
./generate.sh $1 france France
./generate.sh $1 spain Spain
./generate.sh $1 italy Italy
./generate.sh $1 uk UK
./generate.sh $1 croatia Croatia
./generate.sh $1 austria Austria

sleep 3600

./generate.sh $1 portugal Portugal

git add _data
git add *.md
git commit -m "updated data"
git push origin "$now"
curl -v -X POST -H "Authorization: token $1" -H "Content-type: application/vnd.github.v3+json" -d "{ \"title\": \"Data update $now\", \"head\": \"$now\", \"base\": \"master\"}" https://api.github.com/repos/lauripiispanen/github-top/pulls

