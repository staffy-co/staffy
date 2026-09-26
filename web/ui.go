package web

import "embed"

// Assets contains the server-rendered Staffy shell and its self-hosted styles.
//
//go:embed templates/*.html assets/*.css
var Assets embed.FS
