#!/bin/sh

OUTPUT_FILE_NAME=$(echo "$2" | sed 's/ /_/')

github-top --token "$1" --preset "$2" --output yaml --file "$OUTPUT_FILE_NAME.yml"

echo "page: $OUTPUT_FILE_NAME.html\ntitle: $3" | cat - "$OUTPUT_FILE_NAME.yml" > "_data/locations/$OUTPUT_FILE_NAME.yml"
echo "---\ntype: location\nlocation: $OUTPUT_FILE_NAME\nmode: commits\n---" > "$OUTPUT_FILE_NAME.md"
echo "---\ntype: location\nlocation: $OUTPUT_FILE_NAME\nmode: all\n---" > "${OUTPUT_FILE_NAME}_private.md"
echo "---\ntype: location\nlocation: $OUTPUT_FILE_NAME\nmode: contributions\n---" > "${OUTPUT_FILE_NAME}_public.md"

rm "$OUTPUT_FILE_NAME.yml"