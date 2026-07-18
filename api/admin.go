package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/asd1asd00000/svm-panel/database"
	"github.com/asd1asd00000/svm-panel/sshvpn"
)

// registerAdminRoutes مسیرهای پنل مدیریتی (ورود، خروج، عملیات، داشبورد) را ثبت می‌کند
func registerAdminRoutes(token string) {
	http.HandleFunc("/admin/actions", func(w http.ResponseWriter, r *http.Request) {
		if !checkAdminAuth(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		_ = r.ParseForm()
		action := r.FormValue("action")

		switch action {
		case "create_user":
			username := r.FormValue("username")
			password := r.FormValue("password")
			days, _ := strconv.Atoi(r.FormValue("days"))
			vol, _ := strconv.ParseFloat(r.FormValue("volume"), 64)
			
			_, err := database.CreateUser(username, password, days, vol, 0)
			if err != nil {
				database.WriteSystemLog("ERROR", "Web UI Error: Failed to create " + username + " -> " + err.Error())
			} else {
				database.WriteSystemLog("INFO", "Web UI: User created successfully -> " + username)
			}

		case "edit_user":
			username := r.FormValue("username")
			daysChangeStr := r.FormValue("days_change")
			volumeGbStr := r.FormValue("volume_gb")

			if daysChangeStr != "" {
				days, _ := strconv.Atoi(daysChangeStr)
				if days != 0 {
					_ = database.UpdateUserExpiry(username, days)
				}
			}
			if volumeGbStr != "" {
				vol, _ := strconv.ParseFloat(volumeGbStr, 64)
				if vol > 0 {
					_ = database.UpdateUserDataLimit(username, vol)
				}
			}
			database.WriteSystemLog("INFO", "Web UI: User parameters extended/edited -> "+username)

		case "reset_traffic":
			username := r.FormValue("username")
			_ = database.ResetUserDataUsed(username)
			database.WriteSystemLog("INFO", "Web UI: Reset total consumed traffic for -> "+username)

		case "delete_user":
			username := r.FormValue("username")
			_ = database.DeleteUser(username)
			database.WriteSystemLog("INFO", "Web UI: User deleted -> "+username)

		case "edit_node":
			ip := r.FormValue("ip")
			domain := r.FormValue("domain")
			remark := r.FormValue("remark")
			_ = database.UpdateNodeSettings(ip, domain, remark)
			database.WriteSystemLog("INFO", "Web UI: Updated cluster settings for Node -> "+ip)

		case "update_settings":
			_ = database.SetSetting("announcement_url", r.FormValue("announcement_url"))
			_ = database.SetSetting("tutorial_url", r.FormValue("tutorial_url"))
			_ = database.SetSetting("auto_backup_hours", r.FormValue("auto_backup_hours"))
			_ = database.SetSetting("tg_bot_token", r.FormValue("tg_bot_token"))
			_ = database.SetSetting("tg_chat_id", r.FormValue("tg_chat_id"))
			database.WriteSystemLog("INFO", "Web UI: Global core settings updated.")

		case "change_credentials":
			newUsername := r.FormValue("admin_username")
			newPassword := r.FormValue("admin_password")
			if newUsername != "" {
				_ = database.SetSetting("admin_username", newUsername)
			}
			if newPassword != "" {
				_ = database.SetSetting("admin_password", newPassword)
			}
			database.WriteSystemLog("INFO", "Web UI: Admin credentials updated.")
		}

		http.Redirect(w, r, "/admin/dashboard?tab="+r.FormValue("current_tab"), http.StatusSeeOther)
	})

	http.HandleFunc("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			username := r.FormValue("username")
			password := r.FormValue("password")

			dbUser := database.GetSetting("admin_username")
			dbPass := database.GetSetting("admin_password")
			if dbUser == "" { dbUser = "admin" }

			if username == dbUser && password == dbPass {
				http.SetCookie(w, &http.Cookie{
					Name:     "svm_session",
					Value:    "authenticated_admin_session",
					Path:     "/",
					Expires:  time.Now().Add(24 * time.Hour),
					HttpOnly: true,
				})
				http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, "/admin/login?error=true", http.StatusSeeOther)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `
		<!DOCTYPE html>
		<html lang="fa" dir="rtl">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
			<title>ورود به پنل مدیریت SVM</title>
			<script src="https://cdn.tailwindcss.com"></script>
			<link href="https://fonts.googleapis.com/css2?family=Vazirmatn:wght@400;700&display=swap" rel="stylesheet">
			<style>body { font-family: 'Vazirmatn', sans-serif; }</style>
		</head>
		<body class="bg-[#0F172A] text-slate-100 flex items-center justify-center min-h-screen p-4 overflow-x-hidden">
			<div class="w-full max-w-md bg-[#1E293B] border border-slate-700/50 rounded-2xl p-6 shadow-2xl mx-auto">
				<div class="text-center mb-6">
					<h1 class="text-2xl font-bold text-cyan-400 mb-1">svm-panel</h1>
					<p class="text-sm text-slate-400">ورود به سیستم توزیع‌شده مدیریت ترافیک</p>
				</div>
				<form action="/admin/login" method="POST" class="space-y-4 w-full">
					<div>
						<label class="block text-sm font-medium mb-1 text-slate-300">نام کاربری ادمین</label>
						<input type="text" name="username" required class="w-full bg-[#0F172A] border border-slate-600 rounded-xl px-4 py-3 text-slate-100 focus:outline-none focus:border-cyan-500 transition">
					</div>
					<div>
						<label class="block text-sm font-medium mb-1 text-slate-300">رمز عبور</label>
						<input type="password" name="password" required class="w-full bg-[#0F172A] border border-slate-600 rounded-xl px-4 py-3 text-slate-100 focus:outline-none focus:border-cyan-500 transition">
					</div>
					<button type="submit" class="w-full bg-cyan-600 hover:bg-cyan-500 text-white font-bold py-3 rounded-xl transition mt-2 shadow-lg shadow-cyan-600/20">ورود به پنل دسکتاپ و موبایل</button>
				</form>
			</div>
		</body>
		</html>
		`)
	})

	http.HandleFunc("/admin/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "svm_session",
			Value:    "",
			Path:     "/",
			Expires: time.Unix(0, 0),
		})
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	})

	http.HandleFunc("/admin/dashboard", func(w http.ResponseWriter, r *http.Request) {
		if !checkAdminAuth(r) {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		users, _ := database.GetUsers()
		nodes, _ := database.GetNodes()
		
		onlineCount := 0
		liveOnlineMap := make(map[string]bool)
		onlineList := sshvpn.GetOnlineUsersList()
		for _, u := range onlineList {
			liveOnlineMap[u] = true
			onlineCount++
		}

		inactiveCount := 0
		nowUnix := time.Now().Unix()
		for _, u := range users {
			if u.ExpiryDate.Unix() < nowUnix || u.DataUsed >= u.DataLimit {
				inactiveCount++
			}
		}

		var nodeSummaryHTML strings.Builder
		activeNodesCount := 0
		if len(nodes) == 0 {
			nodeSummaryHTML.WriteString(`<span class="text-xs text-slate-500">هیچ نودی متصل نیست</span>`)
		} else {
			for _, n := range nodes {
				isOnline := (nowUnix - n.LastSeen) < 120
				statusColor := "bg-red-500"
				if isOnline {
					statusColor = "bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.6)]"
					activeNodesCount++
				}
				nodeName := n.IP
				if n.CustomRemark != "" {
					nodeName = n.CustomRemark
				}

				nodeSummaryHTML.WriteString(fmt.Sprintf(`
					<div class="flex items-center gap-2 bg-[#0F172A] px-3 py-1.5 rounded-lg border border-slate-700/50">
						<div class="w-2.5 h-2.5 rounded-full %s"></div>
						<span class="text-xs text-slate-300 font-medium">%s</span>
					</div>`, statusColor, nodeName))
			}
		}

		type WebUser struct {
			Username   string `json:"username"`
			DataUsed   int64  `json:"data_used"`
			DataLimit  int64  `json:"data_limit"`
			ExpiryUnix int64  `json:"expiry_unix"`
			LastSeen   int64  `json:"last_seen"`
			SubToken   string `json:"sub_token"`
		}
		
		type WebNode struct {
			IP           string    `json:"IP"`
			LastSeen     int64     `json:"LastSeen"`
			TotalTraffic int64     `json:"TotalTraffic"`
			Domain       string    `json:"Domain"`
			CustomRemark string    `json:"CustomRemark"`
			IsOnline     bool      `json:"IsOnline"`
		}

		webUsers := []WebUser{}
		for _, u := range users {
			webUsers = append(webUsers, WebUser{
				Username:   u.Username,
				DataUsed:   u.DataUsed,
				DataLimit:  u.DataLimit,
				ExpiryUnix: u.ExpiryDate.Unix(),
				LastSeen:   u.LastSeen,
				SubToken:   u.SubToken,
			})
		}

		webNodes := []WebNode{}
		for _, n := range nodes {
			webNodes = append(webNodes, WebNode{
				IP:           n.IP,
				LastSeen:     n.LastSeen,
				TotalTraffic: n.TotalTraffic,
				Domain:       n.Domain,
				CustomRemark: n.CustomRemark,
				IsOnline:     (nowUnix - n.LastSeen) < 120,
			})
		}

		usersJSON, _ := json.Marshal(webUsers)
		nodesJSON, _ := json.Marshal(webNodes)
		liveOnlineJSON, _ := json.Marshal(liveOnlineMap)

		autoBackupHours := database.GetSetting("auto_backup_hours")
		tgToken := database.GetSetting("tg_bot_token")
		tgChat := database.GetSetting("tg_chat_id")
		backupBadgeText := "غیرفعال"
		backupBadgeColor := "bg-red-500/20 text-red-400 border-red-500/30"
		if autoBackupHours != "" && autoBackupHours != "0" && tgToken != "" && tgChat != "" {
			lastStatus := database.GetSetting("last_backup_status")
			if lastStatus == "FAILED" {
				backupBadgeText = "خطا در آخرین ارسال"
				backupBadgeColor = "bg-amber-500/20 text-amber-400 border-amber-500/30"
			} else {
				backupBadgeText = "فعال و منظم"
				backupBadgeColor = "bg-emerald-500/20 text-emerald-400 border-emerald-500/30"
			}
		}

		logContent := "هیچ لاگی ثبت نشده است."
		if logs, err := os.ReadFile("/root/svm-panel/system.log"); err == nil {
			logContent = string(logs)
		}

		currentTab := r.URL.Query().Get("tab")
		if currentTab == "" { currentTab = "dashboard" }

		adminUsername := database.GetSetting("admin_username")
		if adminUsername == "" {
			adminUsername = "admin"
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
		<!DOCTYPE html>
		<html lang="fa" dir="rtl">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
			<title>داشبورد مدیریت کلاستر SVM</title>
			<script src="https://cdn.tailwindcss.com"></script>
			<script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
			<link href="https://fonts.googleapis.com/css2?family=Vazirmatn:wght@400;600;800&display=swap" rel="stylesheet">
			<style>
				body { font-family: 'Vazirmatn', sans-serif; background-color: #0F172A; }
				::-webkit-scrollbar { width: 6px; height: 6px; }
				::-webkit-scrollbar-track { background: #0F172A; }
				::-webkit-scrollbar-thumb { background: #334155; border-radius: 10px; }
			</style>
		</head>
		<body class="text-slate-100 min-h-screen bg-[#0F172A] overflow-x-hidden flex flex-col" x-data='{ activeTab: "%s", mobileMenu: false, editUserModal: false, selectedUser: {}, onlineMap: %s, allUsers: %s }' x-init="$watch('activeTab', val => window.history.replaceState(null, '', '?tab=' + val))">

			<header class="lg:hidden bg-[#1E293B] border-b border-slate-700/50 px-4 py-4 flex items-center justify-between sticky top-0 z-40 w-full flex-shrink-0">
				<h1 class="text-xl font-extrabold text-cyan-400">svm-panel</h1>
				<button @click="mobileMenu = !mobileMenu" class="text-slate-300 p-1 focus:outline-none">
					<svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
					</svg>
				</button>
			</header>

			<div class="flex flex-1 w-full relative">
				
				<aside :class="mobileMenu ? 'translate-x-0' : 'translate-x-full lg:translate-x-0'" class="fixed lg:sticky top-0 right-0 h-[100dvh] lg:h-screen w-64 bg-[#1E293B] border-l border-slate-700/50 p-5 flex flex-col justify-between transition-transform duration-300 ease-in-out z-50 shadow-2xl lg:shadow-none">
					<div>
						<div class="hidden lg:block mb-8 text-center">
							<h1 class="text-2xl font-black text-cyan-400">svm-panel</h1>
							<p class="text-xs text-slate-400 mt-1">مدیریت توزیع‌شده اتصال عمیق</p>
						</div>

						<nav class="space-y-1">
							<button @click="activeTab = 'dashboard'; mobileMenu = false" :class="activeTab === 'dashboard' ? 'bg-cyan-600 text-white' : 'text-slate-300 hover:bg-slate-800'" class="w-full flex items-center gap-3 px-4 py-3 rounded-xl font-medium transition text-right">📊 داشبورد اصلی</button>
							<button @click="activeTab = 'users'; mobileMenu = false" :class="activeTab === 'users' ? 'bg-cyan-600 text-white' : 'text-slate-300 hover:bg-slate-800'" class="w-full flex items-center gap-3 px-4 py-3 rounded-xl font-medium transition text-right">👥 مدیریت کاربران</button>
							<button @click="activeTab = 'nodes'; mobileMenu = false" :class="activeTab === 'nodes' ? 'bg-cyan-600 text-white' : 'text-slate-300 hover:bg-slate-800'" class="w-full flex items-center gap-3 px-4 py-3 rounded-xl font-medium transition text-right">🖥️ نودها</button>
							<!-- تغییر نام منو به تنظیمات -->
							<button @click="activeTab = 'settings'; mobileMenu = false" :class="activeTab === 'settings' ? 'bg-cyan-600 text-white' : 'text-slate-300 hover:bg-slate-800'" class="w-full flex items-center gap-3 px-4 py-3 rounded-xl font-medium transition text-right">⚙️ تنظیمات</button>
						</nav>
					</div>

					<a href="/admin/logout" class="w-full bg-red-500/10 border border-red-500/20 hover:bg-red-500 hover:text-white text-red-400 text-center py-2.5 rounded-xl font-semibold transition mt-auto block">🚪 خروج از حساب</a>
				</aside>

				<div x-show="mobileMenu" @click="mobileMenu = false" class="fixed inset-0 bg-black/60 z-40 lg:hidden" x-transition></div>

				<main class="flex-1 p-4 lg:p-8 w-full min-w-0">
					
					<div class="max-w-7xl mx-auto w-full">
						<div x-show="activeTab === 'dashboard'" class="space-y-6" x-transition>
							<div class="grid grid-cols-2 lg:grid-cols-5 gap-4 lg:gap-6">
								<div class="bg-[#1E293B] border border-slate-700/40 p-4 rounded-2xl flex flex-col justify-between">
									<span class="text-xs lg:text-sm text-slate-400 font-medium">بار پردازنده (CPU)</span>
									<span class="text-xl lg:text-2xl font-extrabold text-slate-100 mt-2">%.1f%%</span>
								</div>
								<div class="bg-[#1E293B] border border-slate-700/40 p-4 rounded-2xl flex flex-col justify-between">
									<span class="text-xs lg:text-sm text-slate-400 font-medium">حافظه موقت (RAM)</span>
									<span class="text-xl lg:text-2xl font-extrabold text-slate-100 mt-2">%.1f%%</span>
								</div>
								<div @click="activeTab = 'users'; $dispatch('set-filter', 'all')" class="bg-[#1E293B] border border-slate-700/40 p-4 rounded-2xl flex flex-col justify-between cursor-pointer hover:bg-slate-800 transition group">
									<span class="text-xs lg:text-sm text-slate-400 font-medium group-hover:text-cyan-300 transition">کل کاربران (مدیریت)</span>
									<span class="text-xl lg:text-2xl font-extrabold text-cyan-400 mt-2">%d کاربر</span>
								</div>
								<div @click="activeTab = 'users'; $dispatch('set-filter', 'online')" class="bg-[#1E293B] border border-slate-700/40 p-4 rounded-2xl flex flex-col justify-between cursor-pointer hover:bg-slate-800 transition group">
									<span class="text-xs lg:text-sm text-slate-400 font-medium group-hover:text-emerald-300 transition">اتصالات آنلاین (فیلتر)</span>
									<span class="text-xl lg:text-2xl font-extrabold text-emerald-400 mt-2">%d آنلاین</span>
								</div>
								<div @click="activeTab = 'users'; $dispatch('set-filter', 'inactive')" class="bg-[#1E293B] border border-slate-700/40 p-4 rounded-2xl flex flex-col justify-between cursor-pointer hover:bg-slate-800 transition group">
									<span class="text-xs lg:text-sm text-slate-400 font-medium group-hover:text-red-300 transition">کاربران غیرفعال (فیلتر)</span>
									<span class="text-xl lg:text-2xl font-extrabold text-red-400 mt-2">%d کاربر</span>
								</div>
							</div>

							<div @click="activeTab = 'nodes'" class="bg-[#1E293B] border border-slate-700/40 p-4 rounded-2xl flex flex-col cursor-pointer hover:bg-slate-800 transition group">
								<div class="flex items-center justify-between mb-4">
									<div class="flex items-center gap-2">
										<span class="text-lg">🖥️</span>
										<span class="text-sm font-medium text-slate-300 group-hover:text-cyan-300 transition">وضعیت نودهای متصل (کلاستر)</span>
									</div>
									<span class="text-xs font-bold border px-3 py-1 rounded-full bg-cyan-500/10 text-cyan-400 border-cyan-500/20">%d از %d فعال</span>
								</div>
								<div class="flex flex-wrap gap-2">
									%s
								</div>
							</div>

							<div @click="activeTab = 'settings'" class="bg-[#1E293B] border border-slate-700/40 p-4 rounded-2xl flex items-center justify-between cursor-pointer hover:bg-slate-800 transition group">
								<div class="flex items-center gap-3">
									<div class="w-2.5 h-2.5 rounded-full bg-cyan-400 animate-pulse"></div>
									<span class="text-sm font-medium text-slate-300 group-hover:text-cyan-300 transition">وضعیت اتوبکاپ ربات تلگرام:</span>
								</div>
								<span class="text-xs font-bold border px-3 py-1 rounded-full %s">%s</span>
							</div>

							<div class="bg-[#1E293B] border border-slate-700/40 rounded-2xl p-4 lg:p-6">
								<h3 class="text-lg font-bold text-slate-200 mb-4 flex items-center gap-2">📜 لاگ‌های امنیتی و عملیاتی سیستم</h3>
								<div class="overflow-x-auto w-full">
									<pre class="bg-[#0F172A] border border-slate-800 text-xs text-slate-300 p-4 rounded-xl min-w-full max-h-96 whitespace-pre-wrap break-words font-mono leading-relaxed">%s</pre>
								</div>
							</div>
						</div>

						<div x-show="activeTab === 'users'" class="space-y-6" x-transition x-data="{
							userSearch: '',
							userFilter: 'all',
							currentPage: 1,
							itemsPerPage: 10,
							get filteredUsers() {
								let result = allUsers;
								if (this.userFilter === 'online') {
									result = result.filter(u => onlineMap[u.username]);
								} else if (this.userFilter === 'inactive') {
									result = result.filter(u => (u.expiry_unix * 1000 <= Date.now() || u.data_used >= u.data_limit));
								}
								if (this.userSearch !== '') {
									result = result.filter(u => u.username.toLowerCase().includes(this.userSearch.toLowerCase()));
								}
								return result;
							},
							get paginatedUsers() {
								let start = (this.currentPage - 1) * this.itemsPerPage;
								let end = start + this.itemsPerPage;
								return this.filteredUsers.slice(start, end);
							},
							get totalPages() {
								return Math.max(1, Math.ceil(this.filteredUsers.length / this.itemsPerPage));
							}
						}" @set-filter.window="userFilter = $event.detail; currentPage = 1">
							<div class="flex flex-col xl:flex-row xl:items-center justify-between gap-4">
								<div class="flex flex-col sm:flex-row flex-wrap items-center gap-3 w-full xl:w-auto">
									<input type="text" x-model="userSearch" @input="currentPage = 1" placeholder="🔍 جستجوی کاربر..." class="bg-[#1E293B] border border-slate-700/60 rounded-xl px-4 py-2.5 text-sm text-slate-100 focus:outline-none focus:border-cyan-500 w-full sm:w-64">
									
									<div class="flex items-center bg-[#1E293B] border border-slate-700/60 rounded-xl p-1 gap-1 w-full sm:w-auto justify-center">
										<button @click="userFilter = 'all'; currentPage = 1" :class="userFilter === 'all' ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-slate-200'" class="px-3 py-1.5 rounded-lg text-xs font-bold transition flex-1 sm:flex-none">همه</button>
										<button @click="userFilter = 'online'; currentPage = 1" :class="userFilter === 'online' ? 'bg-emerald-500/20 text-emerald-400' : 'text-slate-400 hover:text-emerald-400/70'" class="px-3 py-1.5 rounded-lg text-xs font-bold transition flex-1 sm:flex-none">آنلاین</button>
										<button @click="userFilter = 'inactive'; currentPage = 1" :class="userFilter === 'inactive' ? 'bg-red-500/20 text-red-400' : 'text-slate-400 hover:text-red-400/70'" class="px-3 py-1.5 rounded-lg text-xs font-bold transition flex-1 sm:flex-none">غیرفعال</button>
									</div>

									<div class="flex items-center gap-2 w-full sm:w-auto bg-[#1E293B] border border-slate-700/60 rounded-xl px-3 py-1.5 justify-center">
										<span class="text-xs text-slate-400 whitespace-nowrap">تعداد در صفحه:</span>
										<select x-model.number="itemsPerPage" @change="currentPage = 1" class="bg-transparent text-sm text-slate-100 focus:outline-none focus:border-none cursor-pointer py-1">
											<option value="10" class="bg-[#1E293B]">10</option>
											<option value="20" class="bg-[#1E293B]">20</option>
											<option value="50" class="bg-[#1E293B]">50</option>
											<option value="100" class="bg-[#1E293B]">100</option>
										</select>
									</div>
								</div>
								<button @click="editUserModal = true; selectedUser = { mode: 'create' }" class="bg-cyan-600 hover:bg-cyan-500 text-white text-sm font-bold px-5 py-2.5 rounded-xl transition shadow-lg shadow-cyan-600/10 w-full xl:w-auto text-center">+ ساخت اکانت جدید</button>
							</div>

							<div class="bg-[#1E293B] border border-slate-700/40 rounded-2xl overflow-hidden shadow-xl w-full">
								<div class="overflow-x-auto w-full">
									<table class="w-full text-right text-sm whitespace-nowrap min-w-[900px]">
										<thead class="bg-[#0F172A] text-slate-400 text-xs font-semibold uppercase border-b border-slate-700/30">
											<tr>
												<th class="px-4 py-3.5">نام کاربری</th>
												<th class="px-4 py-3.5 text-left">حجم مصرفی / کل</th>
												<th class="px-4 py-3.5">تاریخ انقضا</th>
												<th class="px-4 py-3.5">وضعیت حساب</th>
												<th class="px-4 py-3.5">اتصال</th>
												<th class="px-4 py-3.5 text-center">عملیات مدیریت سیستم</th>
											</tr>
										</thead>
										<tbody class="divide-y divide-slate-800/60">
											<template x-for='user in paginatedUsers' :key="user.username">
												<tr class="hover:bg-slate-800/40 transition">
													<td class="px-4 py-4 font-bold text-slate-200">
														<a :href="'/sub/' + user.sub_token" target="_blank" class="hover:text-cyan-400 underline decoration-slate-600 hover:decoration-cyan-400 transition-colors text-[15px]" x-text="user.username" title="مشاهده صفحه اختصاصی کاربر"></a>
													</td>
													<td class="px-4 py-4" dir="ltr">
														<div class="flex flex-col gap-1.5 w-36 text-left">
															<span class="font-mono text-[11px] text-slate-300" x-text="(user.data_used / (1024*1024)).toFixed(1) + ' MB / ' + (user.data_limit / (1024*1024*1024)).toFixed(2) + ' GB'"></span>
															<div class="w-full bg-slate-700 h-1.5 rounded-full overflow-hidden">
																<div class="h-full transition-all duration-300" :class="(user.expiry_unix * 1000 > Date.now() && user.data_used < user.data_limit) ? 'bg-cyan-500' : 'bg-red-500'" :style="'width: ' + Math.min((user.data_used / user.data_limit) * 100, 100) + '%%'"></div>
															</div>
														</div>
													</td>
													<td class="px-4 py-4">
														<div class="flex flex-col">
															<span class="text-slate-300" x-text="new Date(user.expiry_unix * 1000).toLocaleDateString('fa-IR')"></span>
															<span class="text-[10px] font-bold mt-1" :class="(user.expiry_unix * 1000 > Date.now()) ? 'text-cyan-400' : 'text-red-400'" x-text="(user.expiry_unix * 1000 > Date.now()) ? Math.ceil((user.expiry_unix * 1000 - Date.now()) / (1000 * 60 * 60 * 24)) + ' روز مانده' : 'منقضی شده'"></span>
														</div>
													</td>
													<td class="px-4 py-4">
														<span class="px-2.5 py-1 rounded-lg text-[11px] font-bold whitespace-nowrap" :class="(user.expiry_unix * 1000 > Date.now() && user.data_used < user.data_limit) ? 'bg-emerald-500/10 text-emerald-400' : 'bg-red-500/10 text-red-400'" x-text="(user.expiry_unix * 1000 > Date.now() && user.data_used < user.data_limit) ? '🟢 فعال' : '🔴 غیرفعال'"></span>
													</td>
													<td class="px-4 py-4">
														<span :class="onlineMap[user.username] ? 'text-emerald-400 font-bold' : 'text-slate-500'" x-text="onlineMap[user.username] ? '🟢 آنلاین' : '🔴 آفلاین'"></span>
													</td>
													<td class="px-4 py-4 text-center space-x-2 space-x-reverse">
														<button @click="copySubLink(user.sub_token)" class="bg-cyan-500/10 hover:bg-cyan-500 text-cyan-400 hover:text-white px-2.5 py-1.5 rounded-lg text-xs font-semibold transition">🔗 کپی ساب</button>
														<button @click="selectedUser = user; selectedUser.mode = 'edit'; editUserModal = true" class="bg-yellow-500/10 hover:bg-yellow-500 text-yellow-400 hover:text-slate-900 px-2.5 py-1.5 rounded-lg text-xs font-semibold transition">📝 تمدید</button>
														<form action="/admin/actions" method="POST" onsubmit="return confirm('آیا از صفر کردن ترافیک مصرفی این کاربر مطمئن هستید؟')" class="inline">
															<input type="hidden" name="action" value="reset_traffic">
															<input type="hidden" name="current_tab" value="users">
															<input type="hidden" name="username" :value="user.username">
															<button type="submit" class="bg-orange-500/10 hover:bg-orange-500 text-orange-400 hover:text-white px-2.5 py-1.5 rounded-lg text-xs font-semibold transition">🔄 ریست</button>
														</form>
														<form action="/admin/actions" method="POST" onsubmit="return confirm('آیا از حذف این کاربر مطمئن هستید؟')" class="inline">
															<input type="hidden" name="action" value="delete_user">
															<input type="hidden" name="current_tab" value="users">
															<input type="hidden" name="username" :value="user.username">
															<button type="submit" class="bg-red-500/10 hover:bg-red-500 text-red-400 hover:text-white px-2.5 py-1.5 rounded-lg text-xs font-semibold transition">🗑️ حذف</button>
														</form>
													</td>
												</tr>
											</template>
										</tbody>
									</table>
								</div>
							</div>

							<div class="flex flex-col sm:flex-row items-center justify-between mt-4 bg-[#1E293B] border border-slate-700/40 p-4 rounded-xl gap-4 shadow-sm w-full">
								<div class="text-sm text-slate-400 w-full sm:w-auto text-center sm:text-right">
									نمایش <span class="font-bold text-slate-200" x-text="filteredUsers.length === 0 ? 0 : ((currentPage - 1) * itemsPerPage) + 1"></span>
									تا <span class="font-bold text-slate-200" x-text="Math.min(currentPage * itemsPerPage, filteredUsers.length)"></span>
									از <span class="font-bold text-cyan-400" x-text="filteredUsers.length"></span> کاربر
								</div>
								<div class="flex items-center justify-center gap-2 w-full sm:w-auto">
									<button @click="if(currentPage > 1) currentPage--" :disabled="currentPage === 1" :class="currentPage === 1 ? 'opacity-30 cursor-not-allowed' : 'hover:bg-slate-700'" class="px-4 py-2 bg-[#0F172A] border border-slate-600 rounded-lg text-sm font-medium text-slate-300 transition">قبلی</button>
									
									<div class="flex items-center justify-center bg-[#0F172A] border border-slate-600 rounded-lg px-4 py-2">
										<span class="text-sm font-bold text-slate-200" x-text="'صفحه ' + currentPage + ' از ' + totalPages"></span>
									</div>
									
									<button @click="if(currentPage < totalPages) currentPage++" :disabled="currentPage === totalPages" :class="currentPage === totalPages ? 'opacity-30 cursor-not-allowed' : 'hover:bg-slate-700'" class="px-4 py-2 bg-[#0F172A] border border-slate-600 rounded-lg text-sm font-medium text-slate-300 transition">بعدی</button>
								</div>
							</div>
						</div>

						<div x-show="activeTab === 'nodes'" class="space-y-6" x-transition>
							<div class="bg-[#1E293B] border border-slate-700/40 rounded-2xl overflow-hidden w-full">
								<div class="overflow-x-auto w-full">
									<table class="w-full text-right text-sm whitespace-nowrap min-w-[700px]">
										<thead class="bg-[#0F172A] text-slate-400 text-xs border-b border-slate-700/30">
											<tr>
												<th class="px-4 py-3.5">آی‌پی سرور نود</th>
												<th class="px-4 py-3.5">نام دامنه اختصاصی / DDNS</th>
												<th class="px-4 py-3.5">ریمارک اختصاصی کانفیگ ها</th>
												<th class="px-4 py-3.5">ترافیک کل نود</th>
												<th class="px-4 py-3.5">وضعیت اتصال</th>
												<th class="px-4 py-3.5 text-center">مدیریت لایه اتصال</th>
											</tr>
										</thead>
										<tbody class="divide-y divide-slate-800/60">
											<template x-for='node in %s' :key="node.IP">
												<tr class="hover:bg-slate-800/40 transition">
													<td class="px-4 py-4 font-mono text-cyan-400 font-bold" x-text="node.IP"></td>
													<td class="px-4 py-4 text-slate-300" x-text="node.Domain || 'تنظیم نشده (پیش‌فرض IP)'"></td>
													<td class="px-4 py-4 text-slate-200 font-medium" x-text="node.CustomRemark || 'ژئولوکیشن خودکار'"></td>
													<td class="px-4 py-4 text-xs text-slate-400" x-text="(node.TotalTraffic / (1024*1024*1024)).toFixed(2) + ' GB'"></td>
													<td class="px-4 py-4">
														<span class="px-2.5 py-1 rounded-lg text-[11px] font-bold whitespace-nowrap" :class="node.IsOnline ? 'bg-emerald-500/10 text-emerald-400' : 'bg-red-500/10 text-red-400'" x-text="node.IsOnline ? '🟢 آنلاین' : '🔴 آفلاین'"></span>
													</td>
													<td class="px-4 py-4 text-center space-x-2 space-x-reverse">
														<a :href="'/admin/node-chart?ip=' + node.IP" target="_blank" class="bg-emerald-500/10 hover:bg-emerald-500 text-emerald-400 hover:text-white px-2.5 py-1.5 rounded-lg text-xs font-semibold transition inline-block text-center border border-transparent">📊 نمودار مصرف</a>
														<button @click="selectedUser = node; selectedUser.mode = 'node'; editUserModal = true" class="bg-cyan-500/10 hover:bg-cyan-500 text-cyan-400 hover:text-white px-2.5 py-1.5 rounded-lg text-xs font-semibold transition">📝 ویرایش</button>
													</td>
												</tr>
											</template>
										</tbody>
									</table>
								</div>
							</div>
						</div>

						<div x-show="activeTab === 'settings'" class="space-y-6" x-transition>
							
							<!-- بلوک جدید برای نام کاربری، کلمه عبور و توکن کلاستر -->
							<div class="bg-[#1E293B] border border-slate-700/40 rounded-2xl p-4 lg:p-6 mb-6">
								<h3 class="text-xl font-bold mb-6 text-slate-200 border-b border-slate-700 pb-3">🔐 اطلاعات ورود و اتصال کلاستر</h3>
								
								<div class="mb-6 bg-[#0F172A] p-4 rounded-xl border border-slate-700 flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
									<div>
										<h4 class="font-bold text-cyan-400 mb-1">توکن اتصال کلاستر (Cluster Token)</h4>
										<p class="text-xs text-slate-400">از این توکن برای اتصال نودهای جدید به این سرور اصلی استفاده کنید.</p>
									</div>
									<div class="flex items-center gap-2 w-full md:w-auto">
										<input type="text" readonly value="%s" class="bg-[#1E293B] border border-slate-600 rounded-lg px-3 py-2 text-slate-300 text-sm font-mono w-full md:w-64 outline-none">
										<button onclick="navigator.clipboard.writeText('%s'); alert('توکن کلاستر با موفقیت کپی شد!')" class="bg-cyan-600 hover:bg-cyan-500 text-white px-4 py-2 rounded-lg text-sm font-bold transition whitespace-nowrap">کپی توکن</button>
									</div>
								</div>

								<form action="/admin/actions" method="POST" class="space-y-4">
									<input type="hidden" name="action" value="change_credentials">
									<input type="hidden" name="current_tab" value="settings">
									<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
										<div>
											<label class="block text-sm font-medium mb-1 text-slate-300">نام کاربری جدید</label>
											<input type="text" name="admin_username" value="%s" required class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-4 py-2.5 text-slate-200 focus:outline-none focus:border-cyan-500">
										</div>
										<div>
											<label class="block text-sm font-medium mb-1 text-slate-300">رمز عبور جدید</label>
											<input type="text" name="admin_password" placeholder="برای تغییر رمز وارد کنید..." class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-4 py-2.5 text-slate-200 focus:outline-none focus:border-cyan-500">
										</div>
									</div>
									<button type="submit" class="bg-yellow-600 hover:bg-yellow-500 text-slate-900 font-bold px-6 py-3 rounded-xl transition shadow-lg shadow-yellow-600/10 mt-4 w-full sm:w-auto">💾 تغییر اطلاعات ورود به پنل</button>
								</form>
							</div>

							<div class="bg-[#1E293B] border border-slate-700/40 rounded-2xl p-4 lg:p-6">
								<h3 class="text-xl font-bold mb-6 text-slate-200 border-b border-slate-700 pb-3">🗄️ مدیریت لوکال دیتابیس (بکاپ و بازگردانی)</h3>
								<div class="grid grid-cols-1 md:grid-cols-2 gap-4 lg:gap-6">
									<div class="bg-[#0F172A] p-4 rounded-xl border border-slate-700 flex flex-col justify-between">
										<div>
											<h4 class="font-bold text-emerald-400 mb-2">📥 دانلود فایل بکاپ</h4>
											<p class="text-xs text-slate-400 mb-4">دریافت یک نسخه کامل با فرمت sql از اطلاعات دیتابیس کاربران و نودها.</p>
										</div>
										<a href="/admin/backup/download" class="block text-center bg-emerald-600 hover:bg-emerald-500 text-white font-bold py-2.5 rounded-lg transition text-sm">دانلود بکاپ لوکال</a>
									</div>
									
									<div class="bg-[#0F172A] p-4 rounded-xl border border-slate-700 flex flex-col justify-between">
										<div>
											<h4 class="font-bold text-amber-400 mb-2">📤 آپلود و بازگردانی (Restore)</h4>
											<p class="text-xs text-slate-400 mb-4">هشدار: بازگردانی بکاپ، تمام اطلاعات از جمله یوزر و پسورد ورود به پنل را به زمان بکاپ‌گیری برمی‌گرداند!</p>
										</div>
										<form action="/admin/backup/restore" method="POST" enctype="multipart/form-data" class="flex flex-col gap-3">
											<input type="file" name="backup_file" accept=".sql" required class="text-xs text-slate-300 w-full file:mr-4 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-xs file:font-semibold file:bg-cyan-500/10 file:text-cyan-400 hover:file:bg-cyan-500/20">
											<button type="submit" onclick="return confirm('آیا از بازگردانی این بکاپ اطمینان دارید؟ تمام کاربران فعلی و پسورد پنل به زمان بکاپ برمی‌گردد.')" class="w-full bg-amber-600 hover:bg-amber-500 text-white font-bold py-2.5 rounded-lg transition text-sm">اجرای بازگردانی</button>
										</form>
									</div>
								</div>
							</div>

							<div class="bg-[#1E293B] border border-slate-700/40 rounded-2xl p-4 lg:p-6">
								<h3 class="text-xl font-bold mb-6 text-slate-200 border-b border-slate-700 pb-3">⚙️ تنظیمات لینک‌های وب‌پیج و اتوبکاپ تلگرام</h3>
								<form action="/admin/actions" method="POST" class="space-y-4">
									<input type="hidden" name="action" value="update_settings">
									<input type="hidden" name="current_tab" value="settings">
									<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
										<div>
											<label class="block text-sm font-medium mb-1 text-slate-300">لینک دکمه اطلاعیه‌ها کلاینت</label>
											<input type="text" name="announcement_url" value="%s" class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-4 py-2.5 text-slate-200 focus:outline-none focus:border-cyan-500">
										</div>
										<div>
											<label class="block text-sm font-medium mb-1 text-slate-300">لینک دکمه آموزش‌ها کلاینت</label>
											<input type="text" name="tutorial_url" value="%s" class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-4 py-2.5 text-slate-200 focus:outline-none focus:border-cyan-500">
										</div>
										<div>
											<label class="block text-sm font-medium mb-1 text-slate-300">توکن ربات تلگرام (Bot Token)</label>
											<input type="text" name="tg_bot_token" value="%s" class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-4 py-2.5 text-slate-200 focus:outline-none focus:border-cyan-500">
										</div>
										<div>
											<label class="block text-sm font-medium mb-1 text-slate-300">شناسه چت تلگرام ادمین (Chat ID)</label>
											<input type="text" name="tg_chat_id" value="%s" class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-4 py-2.5 text-slate-200 focus:outline-none focus:border-cyan-500">
										</div>
										<div>
											<label class="block text-sm font-medium mb-1 text-slate-300">دوره تناوب پشتیبان‌گیری (ساعت)</label>
											<input type="number" name="auto_backup_hours" value="%s" class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-4 py-2.5 text-slate-200 focus:outline-none focus:border-cyan-500">
										</div>
									</div>
									<button type="submit" class="bg-emerald-600 hover:bg-emerald-500 text-white font-bold px-6 py-3 rounded-xl transition shadow-lg shadow-emerald-600/10 mt-4 w-full sm:w-auto">💾 ذخیره تغییرات کلی کانفیگ</button>
								</form>
							</div>
						</div>
					</div>
				</main>
			</div>

			<div x-show="editUserModal" class="fixed inset-0 flex items-center justify-center p-4 z-50 animate-fade-in" x-transition style="display: none;">
				<div @click="editUserModal = false" class="absolute inset-0 bg-black/70"></div>
				<div class="bg-[#1E293B] border border-slate-700 rounded-2xl w-full max-w-md p-6 relative z-10 shadow-2xl">
					
					<div x-show="selectedUser.mode === 'create'">
						<h3 class="text-lg font-bold text-cyan-400 mb-4">➕ ساخت اکانت کاربر جدید کلاستر</h3>
						<form action="/admin/actions" method="POST" class="space-y-3">
							<input type="hidden" name="action" value="create_user">
							<input type="hidden" name="current_tab" value="users">
							<div>
								<label class="block text-xs font-medium mb-1 text-slate-400">نام کاربری</label>
								<input type="text" name="username" required class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-3 py-2 text-sm">
							</div>
							<div>
								<label class="block text-xs font-medium mb-1 text-slate-400">رمز عبور</label>
								<input type="text" name="password" required class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-3 py-2 text-sm">
							</div>
							<div class="grid grid-cols-2 gap-2">
								<div>
									<label class="block text-xs font-medium mb-1 text-slate-400">تعداد روز اعتبار</label>
									<input type="number" name="days" value="30" required class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-3 py-2 text-sm">
								</div>
								<div>
									<label class="block text-xs font-medium mb-1 text-slate-400">حجم دوره (GB)</label>
									<input type="number" step="any" name="volume" value="50" required class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-3 py-2 text-sm">
								</div>
							</div>
							<button type="submit" class="w-full bg-cyan-600 hover:bg-cyan-500 py-2.5 rounded-xl font-bold transition text-sm mt-3">تایید و ساخت کاربر</button>
						</form>
					</div>

					<div x-show="selectedUser.mode === 'edit'">
						<h3 class="text-lg font-bold text-yellow-400 mb-4">📝 تمدید دوره و ویرایش ترافیک کاربر</h3>
						<form action="/admin/actions" method="POST" class="space-y-3">
							<input type="hidden" name="action" value="edit_user">
							<input type="hidden" name="current_tab" value="users">
							<input type="hidden" name="username" :value="selectedUser.username">
							<div>
								<label class="block text-xs font-medium mb-1 text-slate-400">کاربر هدف</label>
								<input type="text" :value="selectedUser.username" disabled class="w-full bg-[#0F172A]/50 border border-slate-800 text-slate-500 rounded-xl px-3 py-2 text-sm font-bold">
							</div>
							<div>
								<label class="block text-xs font-medium mb-1 text-slate-400">افزایش/کاهش روزهای اعتبار (مثال: 30 یا 10-)</label>
								<input type="number" name="days_change" placeholder="0" class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-3 py-2 text-sm">
							</div>
							<div>
								<label class="block text-xs font-medium mb-1 text-slate-400">تغییر کل حجم سقف دوره (GB)</label>
								<input type="number" step="any" name="volume_gb" :placeholder="(selectedUser.data_limit / (1024*1024*1024)).toFixed(2)" class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-3 py-2 text-sm">
							</div>
							<button type="submit" class="w-full bg-yellow-600 hover:bg-yellow-500 text-slate-900 py-2.5 rounded-xl font-bold transition text-sm mt-3">💾 ثبت عملیات تمدید اکانت</button>
						</form>
					</div>

					<div x-show="selectedUser.mode === 'node'">
						<h3 class="text-lg font-bold text-cyan-400 mb-4">📝 ویرایش هویت و کانفیگ سرور نود</h3>
						<form action="/admin/actions" method="POST" class="space-y-3">
							<input type="hidden" name="action" value="edit_node">
							<input type="hidden" name="current_tab" value="nodes">
							<input type="hidden" name="ip" :value="selectedUser.IP">
							<div>
								<label class="block text-xs font-medium mb-1 text-slate-400">آی‌پي هدف</label>
								<input type="text" :value="selectedUser.IP" disabled class="w-full bg-[#0F172A]/50 border border-slate-800 text-slate-500 rounded-xl px-3 py-2 text-sm font-mono">
							</div>
							<div>
								<label class="block text-xs font-medium mb-1 text-slate-400">اتصال با دامنه / ساب‌دامنه (به جای IP)</label>
								<input type="text" name="domain" :value="selectedUser.Domain" placeholder="مثال: node2.domain.com" class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-3 py-2 text-sm font-mono">
							</div>
							<div>
								<label class="block text-xs font-medium mb-1 text-slate-400">عنوان و ریمارک نهایی (پرچم و اسم کشور)</label>
								<input type="text" name="remark" :value="selectedUser.CustomRemark" placeholder="مثال: 🇸🇪 Sweden S2" class="w-full bg-[#0F172A] border border-slate-700 rounded-xl px-3 py-2 text-sm">
							</div>
							<button type="submit" class="w-full bg-emerald-600 hover:bg-emerald-500 py-2.5 rounded-xl font-bold transition text-sm mt-3">بروزرسانی نود و لندینگ ساب</button>
						</form>
					</div>

				</div>
			</div>

			<script>
				function copySubLink(token) {
					const subLink = window.location.origin + '/sub/' + token;
					navigator.clipboard.writeText(subLink).then(function() {
						alert("🔗 لینک سابسکریپشن این کاربر با موفقیت کپی شد!");
					});
				}
			</script>

		</body>
		</html>
		`,
			currentTab,
			string(liveOnlineJSON),
			string(usersJSON),
			getSystemCPU(),
			getSystemRAM(),
			len(users),
			onlineCount,
			inactiveCount,
			activeNodesCount,
			len(nodes),
			nodeSummaryHTML.String(),
			backupBadgeColor,
			backupBadgeText,
			logContent,
			string(nodesJSON),
			token, // درج توکن کلاستر برای ورودی
			token, // درج توکن کلاستر برای دکمه کپی
			adminUsername, // درج نام کاربری فعلی ادمین
			database.GetSetting("announcement_url"),
			database.GetSetting("tutorial_url"),
			database.GetSetting("tg_bot_token"),
			database.GetSetting("tg_chat_id"),
			database.GetSetting("auto_backup_hours"),
		)
	})
}
