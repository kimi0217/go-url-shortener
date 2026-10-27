# 短網址服務

[![CI](https://github.com/kimi0217/go-url-shortener/actions/workflows/ci.yml/badge.svg)](https://github.com/kimi0217/go-url-shortener/actions/workflows/ci.yml)

用 Go 標準庫寫的短網址 REST API，沒有任何外部依賴。

## 功能

- 把長網址轉換成 6 碼短網址
- 透過短網址轉址回原本的網址
- 驗證輸入網址，只接受 http / https
- 錯誤一律回傳 JSON 格式

## 需求

Go 1.22 以上（使用 `net/http` 的「方法 + 路徑參數」路由語法）。

## 啟動

```bash
go run ./cmd/server
```

| 環境變數 | 預設值 | 說明 |
|---|---|---|
| `PORT` | `8080` | 監聽的埠號 |
| `BASE_URL` | `http://localhost:$PORT` | 回應中 `short_url` 的前綴；放在反向代理後面時，設成對外的網址 |

## API

### `POST /shorten`：建立短網址

```bash
curl -i -X POST localhost:8080/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://go.dev/doc/"}'
```

```
HTTP/1.1 201 Created
Content-Type: application/json

{"code":"CmeBbR","short_url":"http://localhost:8080/CmeBbR"}
```

### `GET /{code}`：轉址

```bash
curl -i localhost:8080/CmeBbR
```

```
HTTP/1.1 302 Found
Location: https://go.dev/doc/
```

### 錯誤回應

錯誤一律回傳 `{"error": "..."}`，例如：

```
HTTP/1.1 404 Not Found
Content-Type: application/json

{"error":"short code not found"}
```

| 狀態碼 | 情況 |
|---|---|
| 400 | JSON 格式錯誤，或 `url` 不是合法的 http/https 網址 |
| 404 | 短碼不存在 |
| 500 | 內部錯誤（細節只寫入日誌，不回傳給客戶端） |

## 專案結構

```
cmd/server/          進入點：讀取設定、組裝依賴、啟動 http.Server
internal/handler/    HTTP handlers 與 table-driven 測試
internal/store/      Store interface 與記憶體實作
internal/shortener/  短碼產生
```

## 設計說明

- **`Store` interface**：handler 只依賴 interface，測試時可以換成假的實作（例如模擬故障的 store），日後換成資料庫也不用改 handler。所有方法的第一個參數都是 `context.Context`，請求被取消時可以停止工作。
- **併發安全**：記憶體實作使用 `sync.RWMutex`。`Save` 在同一把寫鎖內完成「檢查是否存在」和「寫入」，避免兩個請求同時搶到同一個短碼（check-then-act race）。
- **短碼產生**：使用 `crypto/rand` 產生 6 碼英數字（62⁶ ≈ 570 億種組合），短碼無法被預測或列舉。用 `rand.Int` 取亂數，避免 `byte % 62` 造成的取模偏差。
- **碰撞處理**：`Store.Save` 遇到重複的短碼時回傳 `ErrCodeExists`，handler 重新產生短碼後重試，最多 5 次。
- **輸入驗證**：只接受有 host 的 http/https 網址，擋掉 `javascript:` 等 scheme，避免服務被拿來轉址到惡意連結。請求 body 上限為 1 MiB。
- **302 而非 301**：瀏覽器會永久快取 301，之後的請求就不會再經過這個服務。
- **Server 逾時**：使用自訂的 `http.Server`，設定 `ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout`、`IdleTimeout`，避免慢速客戶端（例如 Slowloris）長時間佔住連線。
- **日誌**：使用 `log/slog` 輸出 JSON 格式。

## 測試

```bash
go test ./...          # 執行所有測試
go test -race ./...    # 加上 race detector
go vet ./...           # 靜態檢查
```

## 已知限制

- 資料存在記憶體，重啟後全部遺失。
- 沒有對應到任何路由的請求（例如 `GET /`、`DELETE /abc`）由 `ServeMux` 直接回傳純文字的 404/405，不是 JSON 格式。
- 同一個網址送出多次，會產生多個不同的短碼。