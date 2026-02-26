//go:build !dashboard

package api

import "embed"

var embeddedDashboard embed.FS

const dashboardEmbedded = false
