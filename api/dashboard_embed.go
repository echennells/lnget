//go:build dashboard

package api

import "embed"

//go:embed all:dashboard_dist
var embeddedDashboard embed.FS

const dashboardEmbedded = true
