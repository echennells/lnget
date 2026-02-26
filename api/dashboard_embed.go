//go:build dashboard

package api

import "embed"

//go:embed dashboard_dist/*
var embeddedDashboard embed.FS

const dashboardEmbedded = true
