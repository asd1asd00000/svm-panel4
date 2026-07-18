package api

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/asd1asd00000/svm-panel/database"
)

// registerBackupRoutes مسیرهای دانلود و بازگردانی بکاپ لوکال دیتابیس را ثبت می‌کند
func registerBackupRoutes() {
	http.HandleFunc("/admin/backup/download", func(w http.ResponseWriter, r *http.Request) {
		if !checkAdminAuth(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		cmd := exec.Command("mysqldump", "-u", "root", "svm_db")
		out, err := cmd.Output()
		if err != nil {
			http.Error(w, "Failed to generate backup: "+err.Error(), http.StatusInternalServerError)
			return
		}
		filename := "svm_backup_" + time.Now().Format("20060102_150405") + ".sql"
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		w.Header().Set("Content-Type", "application/sql")
		w.Write(out)
		database.WriteSystemLog("INFO", "Web UI: Local database backup downloaded.")
	})

	http.HandleFunc("/admin/backup/restore", func(w http.ResponseWriter, r *http.Request) {
		if !checkAdminAuth(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		file, _, err := r.FormFile("backup_file")
		if err != nil {
			http.Error(w, "Failed to read uploaded file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		tempFile, err := os.CreateTemp("/tmp", "restore-*.sql")
		if err != nil {
			http.Error(w, "Failed to create temp file", http.StatusInternalServerError)
			return
		}
		defer os.Remove(tempFile.Name())

		io.Copy(tempFile, file)
		tempFile.Close()

		cmd := exec.Command("mysql", "-u", "root", "svm_db", "-e", "source "+tempFile.Name())
		if err := cmd.Run(); err != nil {
			database.WriteSystemLog("ERROR", "Web UI Error: Local database restore failed -> "+err.Error())
			http.Error(w, "Database restore failed. Check system logs.", http.StatusInternalServerError)
			return
		}

		database.WriteSystemLog("INFO", "Web UI: Database restored successfully from local file.")
		http.Redirect(w, r, "/admin/dashboard?tab=settings", http.StatusSeeOther)
	})
}
