package api

import (
	"log"
	"net/http"
	"strconv"
)

// StartAPIServer تمام مسیرهای HTTP پنل را ثبت کرده و سرور را اجرا می‌کند.
// منطق هر گروه از مسیرها در فایل جداگانه‌ی خودش قرار دارد:
//   - agent.go     → /api/users, /api/usage, /api/online, /api/internal/online (همگام‌سازی نودها)
//   - admin.go     → /admin/login, /admin/logout, /admin/actions, /admin/dashboard
//   - backup.go    → /admin/backup/download, /admin/backup/restore
//   - nodechart.go → /admin/node-chart
//   - sub.go       → /sub/{token}
//   - helpers.go   → توابع کمکی مشترک (formatBytes, getSystemRAM, ...)
func StartAPIServer(port int, token string) {
	registerAgentRoutes(token)
	registerAdminRoutes(token)
	registerBackupRoutes()
	registerNodeChartRoute()
	registerSubRoute()

	log.Printf("Control Panel API Server listening on port %d...\n", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}
