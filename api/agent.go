package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/asd1asd00000/svm-panel4/database"
	"github.com/asd1asd00000/svm-panel4/sshvpn"
)

// registerAgentRoutes مسیرهایی که نودها/کلاینت‌های سرور اصلی برای همگام‌سازی کاربران و ترافیک استفاده می‌کنند را ثبت می‌کند
func registerAgentRoutes(token string) {
	http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		users, err := database.GetUsers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		type SyncUser struct {
			Username   string `json:"username"`
			Password   string `json:"password"`
			DataLimit  int64  `json:"data_limit"`
			DataUsed   int64  `json:"data_used"`
			ExpiryUnix int64  `json:"expiry_unix"`
		}
		var syncList []SyncUser
		for _, u := range users {
			syncList = append(syncList, SyncUser{
				Username:   u.Username,
				Password:   u.Password,
				DataLimit:  u.DataLimit,
				DataUsed:   u.DataUsed,
				ExpiryUnix: u.ExpiryDate.Unix(),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(syncList)
	})

	http.HandleFunc("/api/usage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Query().Get("token") != token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		var payload UsagePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		clientIP := r.Header.Get("X-Real-IP")
		if clientIP == "" {
			clientIP = r.Header.Get("X-Forwarded-For")
		}
		if clientIP == "" {
			var err error
			clientIP, _, err = net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				clientIP = r.RemoteAddr
			}
		}
		clientIP = strings.TrimSpace(strings.Split(clientIP, ",")[0])

		err := database.IncrementUserDataUsed(payload.Username, payload.BytesAdded, clientIP)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Success")
	})

	http.HandleFunc("/api/online", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Query().Get("token") != token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		clientIP := r.Header.Get("X-Real-IP")
		if clientIP == "" {
			clientIP = r.Header.Get("X-Forwarded-For")
		}
		if clientIP == "" {
			var err error
			clientIP, _, err = net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				clientIP = r.RemoteAddr
			}
		}
		clientIP = strings.TrimSpace(strings.Split(clientIP, ",")[0])

		now := time.Now().Unix()
		_ = database.UpdateNodeLastSeen(clientIP, now)

		var onlineUsers []string
		if err := json.NewDecoder(r.Body).Decode(&onlineUsers); err == nil {
			sshvpn.UpdateNodeOnlineStatus(onlineUsers)
			for _, u := range onlineUsers {
				_ = database.UpdateLastSeen(u, now)
				_ = database.IncrementUserDataUsed(u, 0, clientIP)
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/internal/online", func(w http.ResponseWriter, r *http.Request) {
		onlineUsers := sshvpn.GetOnlineUsersList()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(onlineUsers)
	})
}
