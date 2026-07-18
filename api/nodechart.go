package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/asd1asd00000/svm-panel/database"
)

// registerNodeChartRoute مسیر نمایش نمودار مصرف تفکیکی هر نود را ثبت می‌کند
func registerNodeChartRoute() {
	http.HandleFunc("/admin/node-chart", func(w http.ResponseWriter, r *http.Request) {
		if !checkAdminAuth(r) {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		nodeIP := r.URL.Query().Get("ip")
		if nodeIP == "" {
			http.Error(w, "آی‌پی نود نامعتبر است", http.StatusBadRequest)
			return
		}

		nodes, _ := database.GetNodes()
		nodeName := nodeIP
		for _, n := range nodes {
			if n.IP == nodeIP && n.CustomRemark != "" {
				nodeName = n.CustomRemark + " (" + n.IP + ")"
				break
			}
		}

		chartData := database.GetNodeChartData(nodeIP)
		chartDataJSON, _ := json.Marshal(chartData)

		html := fmt.Sprintf(`
		<!DOCTYPE html>
		<html lang="fa" dir="rtl">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
			<title>نمودار ترافیک | %s</title>
			<script src="https://cdn.tailwindcss.com"></script>
			<script src="https://cdn.jsdelivr.net/npm/apexcharts"></script>
			<link href="https://fonts.googleapis.com/css2?family=Vazirmatn:wght@400;600;800&display=swap" rel="stylesheet">
			<style>
				body { font-family: 'Vazirmatn', sans-serif; background-color: #0F172A; }
				.apexcharts-tooltip { background: #1E293B !important; border: 1px solid #334155 !important; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.5) !important; color: #F8FAFC !important; font-family: 'Vazirmatn', sans-serif !important; }
				.apexcharts-tooltip-title { background: #0F172A !important; border-bottom: 1px solid #334155 !important; font-family: 'Vazirmatn', sans-serif !important; font-weight: bold !important; }
			</style>
		</head>
		<body class="text-slate-100 min-h-screen bg-[#0F172A] p-4 lg:p-8 flex items-center justify-center">
			<div class="w-full max-w-4xl bg-[#1E293B] border border-slate-700/50 rounded-2xl p-6 shadow-2xl">
				<div class="flex justify-between items-center border-b border-slate-700 pb-4 mb-6">
					<h2 class="text-xl font-bold text-emerald-400">📊 تفکیک مصرف ترافیک: <span class="text-white">%s</span></h2>
					<button onclick="window.close()" class="bg-red-500/10 hover:bg-red-500 text-red-400 hover:text-white px-4 py-2 rounded-lg text-sm font-semibold transition">بستن صفحه ✖</button>
				</div>
				<div id="nodeApexChart" class="w-full h-[450px]"></div>
			</div>

			<script>
				var options = {
					series: [{
						name: 'مصرف ترافیک (GB)',
						data: %s
					}],
					chart: {
						type: 'bar',
						height: 450,
						toolbar: { show: false },
						fontFamily: 'Vazirmatn, Tahoma, sans-serif',
						foreColor: '#94A3B8',
						background: 'transparent'
					},
					plotOptions: {
						bar: { borderRadius: 4, columnWidth: '45%%' }
					},
					dataLabels: { enabled: false },
					xaxis: {
						categories: ['1 ساعت', '2 ساعت', '6 ساعت', '12 ساعت', '24 ساعت', 'امروز', 'دیروز', '3 روز', '1 هفته', '30 روز', '90 روز'],
						axisBorder: { show: false },
						axisTicks: { show: false },
						labels: { style: { fontSize: '12px' } }
					},
					colors: ['#10B981'],
					theme: { mode: 'dark' },
					grid: { borderColor: '#334155', strokeDashArray: 4 },
					tooltip: {
						theme: 'dark',
						y: { formatter: function (val) { return val.toFixed(3) + " GB" } }
					}
				};
				var chart = new ApexCharts(document.querySelector("#nodeApexChart"), options);
				chart.render();
			</script>
		</body>
		</html>
		`, nodeName, nodeName, string(chartDataJSON))

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})
}
