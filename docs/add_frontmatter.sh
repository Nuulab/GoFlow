#!/bin/bash

for file in content/docs/guide/*.md content/docs/api/*.md content/docs/examples/*.md; do
  title=$(grep -m1 "^# " "$file" | sed 's/^# //')
  if [ -n "$title" ]; then
    echo "---" > /tmp/fm_temp.md
    echo "title: $title" >> /tmp/fm_temp.md
    echo "---" >> /tmp/fm_temp.md
    echo "" >> /tmp/fm_temp.md
    tail -n +2 "$file" >> /tmp/fm_temp.md
    mv /tmp/fm_temp.md "$file"
    echo "Added frontmatter to: $file"
  fi
done
