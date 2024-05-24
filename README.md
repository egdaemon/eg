### EG daemon

the only developer experience focused ci/cd system. coming soon.

gomarkdoc --output ".docs/{{.ImportPath}}.md" --exclude-dirs="./vendor/..." --exclude-dirs="./.eg/..." --exclude-dirs="./runtime/wasi/internal/..." --exclude-dirs="./.test/..." ./runtime/...
