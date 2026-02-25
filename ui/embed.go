package ui

import "embed"

// Содержимое frontend/dist. Для сборки: cd frontend && npm run build, затем копировать в ui/frontend/dist.
//go:embed all:frontend/dist
var Assets embed.FS
