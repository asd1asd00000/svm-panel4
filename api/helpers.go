package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type UsagePayload struct {
	Username   string `json:"username"`
	BytesAdded int64  `json:"bytes_added"`
}

type NPVConfig struct {
	SSHConfigType       string `json:"sshConfigType"`
	Remarks             string `json:"remarks"`
	SSHHost             string `json:"sshHost"`
	SSHPort             int    `json:"sshPort"`
	SSHUsername         string `json:"sshUsername"`
	SSHPassword         string `json:"sshPassword"`
	UDPGWPort           int    `json:"udpgwPort"`
	UDPGWTransparentDNS bool   `json:"udpgwTransparentDNS"`
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit { return fmt.Sprintf("%d B", b) }
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func getSystemRAM() float64 {
	data, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil { return 0 }
	var total, free, cached, buffers int64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 { continue }
		val, _ := strconv.ParseInt(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:": total = val
		case "MemFree:": free = val
		case "Cached:": cached = val
		case "Buffers:": buffers = val
		}
	}
	if total == 0 { return 0 }
	used := total - (free + cached + buffers)
	return (float64(used) / float64(total)) * 100
}

func getSystemCPU() float64 {
	data, err := ioutil.ReadFile("/proc/stat")
	if err != nil { return 0 }
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 { return 0 }
	fields := strings.Fields(lines[0])
	if len(fields) < 5 { return 0 }
	var total int64
	for i := 1; i < len(fields); i++ {
		v, _ := strconv.ParseInt(fields[i], 10, 64)
		total += v
	}
	idle, _ := strconv.ParseInt(fields[4], 10, 64)
	if total == 0 { return 0 }
	return float64(total-idle) / float64(total) * 100
}

func getCountryFlag(ip string) (string, string) {
	if ip == "Main-Server" || ip == "127.0.0.1" || strings.HasPrefix(ip, "192.168") || strings.HasPrefix(ip, "10.") {
		return "Local", "🌍"
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://ip-api.com/json/" + ip)
	if err != nil { return "Unknown", "🌍" }
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(body, &result)
	countryCode, ok := result["countryCode"].(string)
	if !ok { return "Unknown", "🌍" }
	countryName, _ := result["country"].(string)
	flag := ""
	if len(countryCode) == 2 {
		flag = string([]rune{rune(countryCode[0]) + 127397, rune(countryCode[1]) + 127397})
	}
	return countryName, flag
}

func checkAdminAuth(r *http.Request) bool {
	cookie, err := r.Cookie("svm_session")
	if err != nil { return false }
	return cookie.Value == "authenticated_admin_session"
}
