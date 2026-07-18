package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

type User struct {
	ID         int
	Username   string
	Password   string
	ExpiryDate time.Time
	DataLimit  int64
	DataUsed   int64
	LastSeen   int64
	SubToken   string
	UdpgwPort  int
}

type TrafficStats struct {
	H1        float64
	H2        float64
	H6        float64
	H12       float64
	Today     float64
	Yesterday float64
	ThisWeek  float64
	ThisMonth float64
	M3        float64
}

type Node struct {
	IP           string
	LastSeen     int64
	TotalTraffic int64
	Name         string
	Domain       string
	CustomRemark string
}

func ConnectDB() {
	tempDSN := "root:@tcp(127.0.0.1:3306)/?parseTime=true"
	tempDB, err := sql.Open("mysql", tempDSN)
	if err != nil {
		log.Fatalf("Error connecting to MySQL for DB creation: %v\n", err)
	}
	_, err = tempDB.Exec("CREATE DATABASE IF NOT EXISTS svm_db")
	if err != nil {
		log.Fatalf("Error creating database: %v\n", err)
	}
	tempDB.Close()

	dsn := "root:@tcp(127.0.0.1:3306)/svm_db?parseTime=true"
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Error connecting to database server: %v\n", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatalf("Database server is unreachable: %v\n", err)
	}

	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(255) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			expiry_date DATETIME,
			data_limit BIGINT DEFAULT 0,
			data_used BIGINT DEFAULT 0,
			last_seen BIGINT DEFAULT 0,
			sub_token VARCHAR(64),
			udpgw_port INT DEFAULT 7301
		);`,
		`CREATE TABLE IF NOT EXISTS traffic_logs (
			username VARCHAR(50) NOT NULL,
			log_date DATE NOT NULL,
			log_hour INT NOT NULL,
			node_ip VARCHAR(50) NOT NULL DEFAULT 'Main-Server',
			bytes_used BIGINT DEFAULT 0,
			PRIMARY KEY (username, log_date, log_hour, node_ip)
		);`,
		`CREATE TABLE IF NOT EXISTS nodes (
			ip VARCHAR(50) PRIMARY KEY,
			last_seen BIGINT DEFAULT 0,
			total_traffic BIGINT DEFAULT 0,
			name VARCHAR(50) DEFAULT '',
			domain VARCHAR(255) DEFAULT '',
			custom_remark VARCHAR(150) DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS node_traffic_logs (
			ip VARCHAR(50) NOT NULL,
			log_date DATE NOT NULL,
			log_hour INT NOT NULL,
			bytes_used BIGINT DEFAULT 0,
			PRIMARY KEY (ip, log_date, log_hour)
		);`,
		`CREATE TABLE IF NOT EXISTS settings (
			key_name VARCHAR(255) PRIMARY KEY,
			key_value TEXT
		);`,
	}

	for _, query := range tables {
		_, err = DB.Exec(query)
		if err != nil {
			log.Fatalf("Error initializing table: %v\n", err)
		}
	}
}

func generateSubToken() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func SetSetting(key, value string) error {
	_, err := DB.Exec("REPLACE INTO settings (key_name, key_value) VALUES (?, ?)", key, value)
	return err
}

func GetSetting(key string) string {
	var val string
	err := DB.QueryRow("SELECT key_value FROM settings WHERE key_name = ?", key).Scan(&val)
	if err != nil {
		return ""
	}
	return val
}

func SendTelegramMessage(text string) {
	botToken := GetSetting("tg_bot_token")
	chatID := GetSetting("tg_chat_id")
	if botToken == "" || chatID == "" {
		return
	}
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	resp, err := http.PostForm(apiURL, url.Values{"chat_id": {chatID}, "text": {text}})
	if err == nil {
		resp.Body.Close()
	}
}

func WriteSystemLog(level, msg string) {
	f, err := os.OpenFile("/root/svm-panel/system.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		f.WriteString(time.Now().Format("2006-01-02 15:04:05") + " | [" + level + "] " + msg + "\n")
		f.Close()
	}
}

func StartLogMaintenanceDaemon() {
	cleanLogs()
	for {
		time.Sleep(6 * time.Hour)
		cleanLogs()
	}
}

func cleanLogs() {
	filePath := "/root/svm-panel/system.log"
	os.Remove("/root/svm-panel/backup.log") 
	
	content, err := os.ReadFile(filePath)
	if err != nil { return }

	lines := strings.Split(string(content), "\n")
	var retainedLogs []string
	now := time.Now()

	for _, line := range lines {
		if strings.TrimSpace(line) == "" { continue }
		parts := strings.SplitN(line, " | ", 2)
		if len(parts) < 2 { continue }

		logTime, err := time.Parse("2006-01-02 15:04:05", parts[0])
		if err != nil { continue }

		ageHours := now.Sub(logTime).Hours()
		isError := strings.Contains(parts[1], "[ERROR]") || strings.Contains(parts[1], "FAILED")
		isSuccess := strings.Contains(parts[1], "[INFO]") || strings.Contains(parts[1], "SUCCESS")

		if isSuccess && ageHours <= 24 {
			retainedLogs = append(retainedLogs, line)
		} else if isError && ageHours <= (30*24) { 
			retainedLogs = append(retainedLogs, line)
		} else if !isSuccess && !isError && ageHours <= 24 { 
			retainedLogs = append(retainedLogs, line)
		}
	}

	output := strings.Join(retainedLogs, "\n")
	if len(retainedLogs) > 0 { output += "\n" }
	_ = os.WriteFile(filePath, []byte(output), 0644)
}

func RunAdvancedBackup(zipPass string) error {
	botToken := GetSetting("tg_bot_token")
	chatID := GetSetting("tg_chat_id")
	timestamp := time.Now().Format("20060102_150405")
	sqlFile := fmt.Sprintf("/tmp/svm_backup_%s.sql", timestamp)
	zipFile := fmt.Sprintf("/root/svm_backup_%s.zip", timestamp)

	cmdDump := exec.Command("mysqldump", "-u", "root", "svm_db")
	outFile, err := os.Create(sqlFile)
	if err != nil { return err }
	cmdDump.Stdout = outFile
	if err := cmdDump.Run(); err != nil {
		outFile.Close()
		return err
	}
	outFile.Close()

	var cmdZip *exec.Cmd
	if zipPass != "" {
		cmdZip = exec.Command("zip", "-P", zipPass, "-j", zipFile, sqlFile)
	} else {
		cmdZip = exec.Command("zip", "-j", zipFile, sqlFile)
	}
	if err := cmdZip.Run(); err != nil { return err }
	os.Remove(sqlFile)

	if botToken != "" && chatID != "" {
		tgURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", botToken)
		out, err := exec.Command("curl", "-s", "-F", "chat_id="+chatID, "-F", "document=@"+zipFile, tgURL).CombinedOutput()
		if err != nil { return fmt.Errorf("curl failed: %v", err) }
		if !strings.Contains(string(out), "\"ok\":true") {
			return fmt.Errorf("telegram api error: %s", string(out))
		}
	} else {
		return fmt.Errorf("telegram bot token or chat id is missing")
	}
	return nil
}

func RestoreBackup(zipPath, password string) error {
	var extractCmd string
	if password != "" {
		extractCmd = fmt.Sprintf("unzip -p -P %s %s > /tmp/restore.sql && mysql -u root svm_db < /tmp/restore.sql", password, zipPath)
	} else {
		extractCmd = fmt.Sprintf("unzip -p %s > /tmp/restore.sql && mysql -u root svm_db < /tmp/restore.sql", zipPath)
	}
	cmd := exec.Command("sh", "-c", extractCmd)
	err := cmd.Run()
	os.Remove("/tmp/restore.sql")
	return err
}

func StartAutoBackupDaemon() {
	for {
		time.Sleep(1 * time.Minute)
		intervalStr := GetSetting("auto_backup_hours")
		interval, err := strconv.Atoi(intervalStr)
		if err != nil || interval <= 0 { continue }
		lastBackupStr := GetSetting("last_auto_backup_unix")
		lastBackup, _ := strconv.ParseInt(lastBackupStr, 10, 64)
		
		if time.Now().Unix()-lastBackup >= int64(interval*3600) {
			err := RunAdvancedBackup(GetSetting("zip_password"))
			if err != nil {
				SetSetting("last_backup_status", "FAILED")
				WriteSystemLog("ERROR", "Auto-Backup Failed: "+err.Error())
				SendTelegramMessage("❌ SVM Panel Auto-Backup Failed!\nTime: " + time.Now().Format("2006-01-02 15:04:05") + "\nError: " + err.Error())
			} else {
				SetSetting("last_backup_status", "SUCCESS")
				WriteSystemLog("INFO", "Auto-Backup created and sent to Telegram successfully.")
			}
			_ = SetSetting("last_auto_backup_unix", fmt.Sprintf("%d", time.Now().Unix()))
		}
	}
}

func CreateUser(username, password string, days int, volumeGB float64, udpgwPort int) (string, error) {
	expiryDate := time.Now().AddDate(0, 0, days)
	dataLimit := int64(volumeGB * 1024 * 1024 * 1024)
	token := generateSubToken()
	_, err := DB.Exec("INSERT INTO users (username, password, expiry_date, data_limit, data_used, udpgw_port, sub_token) VALUES (?, ?, ?, ?, 0, ?, ?)", username, password, expiryDate, dataLimit, udpgwPort, token)
	return token, err
}

func DeleteUser(username string) error {
	_, err := DB.Exec("DELETE FROM users WHERE username = ?", username)
	return err
}

func UpdateUserExpiry(username string, addDays int) error {
	_, err := DB.Exec("UPDATE users SET expiry_date = DATE_ADD(expiry_date, INTERVAL ? DAY) WHERE username = ?", addDays, username)
	return err
}

func UpdateUserDataLimit(username string, newVolumeGB float64) error {
	dataLimit := int64(newVolumeGB * 1024 * 1024 * 1024)
	_, err := DB.Exec("UPDATE users SET data_limit = ? WHERE username = ?", dataLimit, username)
	return err
}

func GetUsers() ([]User, error) {
	rows, err := DB.Query("SELECT id, username, password, expiry_date, data_limit, data_used, IFNULL(last_seen, 0), IFNULL(sub_token, ''), IFNULL(udpgw_port, 7301) FROM users")
	if err != nil { return nil, err }
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Password, &u.ExpiryDate, &u.DataLimit, &u.DataUsed, &u.LastSeen, &u.SubToken, &u.UdpgwPort); err == nil {
			users = append(users, u)
		}
	}
	return users, nil
}

func GetUserBySubToken(token string) (User, error) {
	var u User
	err := DB.QueryRow("SELECT id, username, password, expiry_date, data_limit, data_used, IFNULL(last_seen, 0), IFNULL(sub_token, ''), IFNULL(udpgw_port, 7301) FROM users WHERE sub_token = ?", token).
		Scan(&u.ID, &u.Username, &u.Password, &u.ExpiryDate, &u.DataLimit, &u.DataUsed, &u.LastSeen, &u.SubToken, &u.UdpgwPort)
	return u, err
}

// 🔴 راه‌حل هوشمندانه: از این به بعد تمام ترافیک‌ها همزمان در جدول نودها هم لاگ می‌شوند
func IncrementUserDataUsed(username string, bytes int64, nodeIP string) error {
	if nodeIP == "" { nodeIP = "Main-Server" }
	_, err := DB.Exec("UPDATE users SET data_used = data_used + ? WHERE username = ?", bytes, username)
	if err != nil { return err }
	
	loc := time.FixedZone("IRST", 12600)
	now := time.Now().In(loc)
	logDate := now.Format("2006-01-02")
	logHour := now.Hour()
	
	// لاگ ترافیک کاربر
	_, _ = DB.Exec("INSERT INTO traffic_logs (username, log_date, log_hour, node_ip, bytes_used) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE bytes_used = bytes_used + ?", username, logDate, logHour, nodeIP, bytes, bytes)
	
	// لاگ ترافیک نود (ترافیک Main Server در sshvpn هم از همینجا لاگ می‌شود!)
	_, _ = DB.Exec("INSERT INTO nodes (ip, total_traffic, last_seen) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE total_traffic = total_traffic + ?, last_seen = ?", nodeIP, bytes, now.Unix(), bytes, now.Unix())
	_, _ = DB.Exec("INSERT INTO node_traffic_logs (ip, log_date, log_hour, bytes_used) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE bytes_used = bytes_used + ?", nodeIP, logDate, logHour, bytes, bytes)

	return nil
}

func ResetUserDataUsed(username string) error {
	_, err := DB.Exec("UPDATE users SET data_used = 0 WHERE username = ?", username)
	return err
}

func ResetUserExpiry(username string, daysFromToday int) error {
	expiryDate := time.Now().AddDate(0, 0, daysFromToday)
	_, err := DB.Exec("UPDATE users SET expiry_date = ? WHERE username = ?", expiryDate, username)
	return err
}

func UpdateLastSeen(username string, timestamp int64) error {
	_, err := DB.Exec("UPDATE users SET last_seen = ? WHERE username = ?", timestamp, username)
	return err
}

func GetUserTrafficStats(username string) TrafficStats {
	var stats TrafficStats
	loc := time.FixedZone("IRST", 12600)
	now := time.Now().In(loc)
	todayStr := now.Format("2006-01-02")
	yesterdayStr := now.AddDate(0, 0, -1).Format("2006-01-02")
	weekAgoStr := now.AddDate(0, 0, -7).Format("2006-01-02")
	monthAgoStr := now.AddDate(0, -1, 0).Format("2006-01-02")
	month3AgoStr := now.AddDate(0, -3, 0).Format("2006-01-02")

	getSum := func(condition string, args ...interface{}) float64 {
		query := fmt.Sprintf("SELECT IFNULL(SUM(bytes_used), 0) FROM traffic_logs WHERE username = ? AND %s", condition)
		var bytes int64
		fullArgs := append([]interface{}{username}, args...)
		_ = DB.QueryRow(query, fullArgs...).Scan(&bytes)
		return float64(bytes) / (1024 * 1024 * 1024)
	}

	getHoursSum := func(hours int) float64 {
		t := now.Add(-time.Duration(hours) * time.Hour)
		tStr := t.Format("2006-01-02 15:04:05")
		var bytes int64
		_ = DB.QueryRow("SELECT IFNULL(SUM(bytes_used), 0) FROM traffic_logs WHERE username = ? AND TIMESTAMP(log_date, MAKETIME(log_hour, 0, 0)) >= ?", username, tStr).Scan(&bytes)
		return float64(bytes) / (1024 * 1024 * 1024)
	}

	stats.H1 = getHoursSum(1)
	stats.H2 = getHoursSum(2)
	stats.H6 = getHoursSum(6)
	stats.H12 = getHoursSum(12)
	stats.Today = getSum("log_date = ?", todayStr)
	stats.Yesterday = getSum("log_date = ?", yesterdayStr)
	stats.ThisWeek = getSum("log_date >= ?", weekAgoStr)
	stats.ThisMonth = getSum("log_date >= ?", monthAgoStr)
	stats.M3 = getSum("log_date >= ?", month3AgoStr)
	return stats
}

func UpdateNodeLastSeen(ip string, timestamp int64) error {
	_, err := DB.Exec("INSERT INTO nodes (ip, last_seen) VALUES (?, ?) ON DUPLICATE KEY UPDATE last_seen = ?", ip, timestamp, timestamp)
	return err
}

func UpdateNodeSettings(ip, domain, remark string) error {
	query := `INSERT INTO nodes (ip, name, domain, custom_remark) 
	          VALUES (?, ?, ?, ?) 
	          ON DUPLICATE KEY UPDATE domain = ?, custom_remark = ?, name = ?`
	_, err := DB.Exec(query, ip, remark, domain, remark, domain, remark, remark)
	return err
}

// تابع اضافی که حذف شد تا جلوی ثبت دوبرابری ترافیک نودها گرفته شود
func IncrementNodeTraffic(ip string, bytes int64) error {
	return nil
}

func GetNodes() ([]Node, error) {
	rows, err := DB.Query("SELECT ip, last_seen, total_traffic, IFNULL(name, ''), IFNULL(domain, ''), IFNULL(custom_remark, '') FROM nodes")
	if err != nil { return nil, err }
	defer rows.Close()
	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.IP, &n.LastSeen, &n.TotalTraffic, &n.Name, &n.Domain, &n.CustomRemark); err == nil {
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

func GetNodeTrafficStats(ip string) TrafficStats {
	var stats TrafficStats
	return stats
}

func GetNodeChartData(ip string) []float64 {
	loc := time.FixedZone("IRST", 12600)
	now := time.Now().In(loc)

	fetchHourly := func(hours int) float64 {
		tStr := now.Add(time.Duration(-hours) * time.Hour).Format("2006-01-02 15:00:00")
		var b int64
		_ = DB.QueryRow("SELECT IFNULL(SUM(bytes_used), 0) FROM node_traffic_logs WHERE ip = ? AND TIMESTAMP(log_date, MAKETIME(log_hour, 0, 0)) >= ?", ip, tStr).Scan(&b)
		return float64(b) / (1024 * 1024 * 1024)
	}

	fetchDaily := func(days int) float64 {
		tStr := now.AddDate(0, 0, -days).Format("2006-01-02")
		var b int64
		_ = DB.QueryRow("SELECT IFNULL(SUM(bytes_used), 0) FROM node_traffic_logs WHERE ip = ? AND log_date >= ?", ip, tStr).Scan(&b)
		return float64(b) / (1024 * 1024 * 1024)
	}

	fetchExactDay := func(daysAgo int) float64 {
		tStr := now.AddDate(0, 0, -daysAgo).Format("2006-01-02")
		var b int64
		_ = DB.QueryRow("SELECT IFNULL(SUM(bytes_used), 0) FROM node_traffic_logs WHERE ip = ? AND log_date = ?", ip, tStr).Scan(&b)
		return float64(b) / (1024 * 1024 * 1024)
	}

	return []float64{
		fetchHourly(1),
		fetchHourly(2),
		fetchHourly(6),
		fetchHourly(12),
		fetchHourly(24),
		fetchExactDay(0),
		fetchExactDay(1),
		fetchDaily(3),   
		fetchDaily(7),   
		fetchDaily(30),  
		fetchDaily(90),  
	}
}
