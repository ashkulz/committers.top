#!/bin/sh

if [ -z "$1" ]; then
    exit 1
fi

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
./generate.sh $1 estonia Estonia
./generate.sh $1 denmark Denmark
./generate.sh $1 france France
./generate.sh $1 spain Spain
./generate.sh $1 italy Italy
./generate.sh $1 uk UK
./generate.sh $1 croatia Croatia
./generate.sh $1 austria Austria
./generate.sh $1 portugal Portugal
./generate.sh $1 worldwide Worldwide
./generate.sh $1 china China
./generate.sh $1 india India
./generate.sh $1 indonesia Indonesia
./generate.sh $1 pakistan Pakistan
./generate.sh $1 brazil Brazil
./generate.sh $1 nigeria Nigeria

sleep 1800

./generate.sh $1 bangladesh Bangladesh
./generate.sh $1 mexico Mexico
./generate.sh $1 philippines Philippines
./generate.sh $1 luxembourg Luxembourg
./generate.sh $1 egypt Egypt
./generate.sh $1 ethiopia Ethiopia
./generate.sh $1 vietnam Vietnam
./generate.sh $1 iran Iran
./generate.sh $1 congo Congo
./generate.sh $1 turkey Turkey
./generate.sh $1 israel Israel
./generate.sh $1 thailand Thailand
./generate.sh $1 "south africa" "South Africa"
./generate.sh $1 myanmar Myanmar
./generate.sh $1 tanzania Tanzania
./generate.sh $1 "south korea" "Republic of Korea"
./generate.sh $1 colombia Colombia
./generate.sh $1 kenya Kenya
./generate.sh $1 argentina Argentina
./generate.sh $1 algeria Algeria
./generate.sh $1 sudan Sudan
./generate.sh $1 poland Poland
./generate.sh $1 canada Canada

sleep 1800

./generate.sh $1 australia Australia
./generate.sh $1 "new zealand" "New Zealand"
./generate.sh $1 belgium Belgium
./generate.sh $1 greece Greece
./generate.sh $1 peru Peru
./generate.sh $1 hungary Hungary
./generate.sh $1 albania Albania
./generate.sh $1 uganda Uganda
./generate.sh $1 zambia Zambia
./generate.sh $1 "sri lanka" "Sri Lanka"
./generate.sh $1 singapore Singapore
./generate.sh $1 latvia Latvia
./generate.sh $1 romania Romania
./generate.sh $1 belarus Belarus
./generate.sh $1 malta Malta
./generate.sh $1 rwanda Rwanda
./generate.sh $1 "saudi arabia" "Saudi Arabia"
./generate.sh $1 morocco Morocco
./generate.sh $1 uzbekistan Uzbekistan
./generate.sh $1 malaysia Malaysia
./generate.sh $1 afghanistan Afghanistan
./generate.sh $1 venezuela Venezuela

sleep 1800

./generate.sh $1 ghana Ghana
./generate.sh $1 angola Angola
./generate.sh $1 nepal Nepal
./generate.sh $1 yemen Yemen
./generate.sh $1 mozambique Mozambique
./generate.sh $1 "ivory coast" "Ivory Coast"
./generate.sh $1 cameroon Cameroon
./generate.sh $1 taiwan Taiwan
./generate.sh $1 niger Niger
./generate.sh $1 "burkina faso" "Burkina Faso"
./generate.sh $1 mali Mali
./generate.sh $1 malawi Malawi
./generate.sh $1 chile Chile
./generate.sh $1 kazakhstan Kazakhstan
./generate.sh $1 guatemala Guatemala
./generate.sh $1 ecuador Ecuador
./generate.sh $1 syria Syria
./generate.sh $1 cambodia Cambodia
./generate.sh $1 senegal Senegal
./generate.sh $1 chad Chad
./generate.sh $1 somalia Somalia
./generate.sh $1 zimbabwe Zimbabwe
./generate.sh $1 guinea Guinea
./generate.sh $1 benin Benin

sleep 1800

./generate.sh $1 haiti Haiti
./generate.sh $1 cuba Cuba
./generate.sh $1 bolivia Bolivia
./generate.sh $1 tunisia Tunisia
./generate.sh $1 "south sudan" "South Sudan"
./generate.sh $1 burundi Burundi
./generate.sh $1 "dominican republic" "Dominican Republic"
./generate.sh $1 "czech republic" "Czech Republic"
./generate.sh $1 jordan Jordan
./generate.sh $1 azerbaijan Azerbaijan
./generate.sh $1 uae UAE
./generate.sh $1 honduras Honduras
./generate.sh $1 tajikistan Tajikistan
./generate.sh $1 "papua new guinea" "Papua New Guinea"
./generate.sh $1 serbia Serbia
./generate.sh $1 switzerland Switzerland
./generate.sh $1 togo Togo
./generate.sh $1 "sierra leone" "Sierra Leone"
./generate.sh $1 "hong kong" "Hong Kong"
./generate.sh $1 "el salvador" "El Salvador"
./generate.sh $1 kyrgyzstan Kyrgyzstan
./generate.sh $1 nicaragua Nicaragua
./generate.sh $1 turkmenistan Turkmenistan
./generate.sh $1 paraguay Paraguay

sleep 1800

./generate.sh $1 laos Laos
./generate.sh $1 bulgaria Bulgaria
./generate.sh $1 lebanon Lebanon
./generate.sh $1 libya Libya
./generate.sh $1 slovakia Slovakia
./generate.sh $1 lithuania Lithuania
./generate.sh $1 ireland Ireland
./generate.sh $1 "united states" "United States"

git add _data
git add *.md
git commit -m "updated data"
git push origin "$now"
curl -v -X POST -H "Authorization: token $1" -H "Content-type: application/vnd.github.v3+json" -d "{ \"title\": \"Data update $now\", \"head\": \"$now\", \"base\": \"master\"}" https://api.github.com/repos/lauripiispanen/github-top/pulls

