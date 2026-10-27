# 短網址服務

[![CI](https://github.com/kimi0217/go-url-shortener/actions/workflows/ci.yml/badge.svg)](https://github.com/kimi0217/go-url-shortener/actions/workflows/ci.yml)

**用 Go 標準庫實作的短網址 REST API。送出長網址，取得 6 碼短網址。**

> A URL shortener REST API written in Go with only the standard library — no frameworks and no external dependencies.
> It uses Go 1.22's method-and-wildcard routing, a `Store` interface backed by a concurrency-safe in-memory map, and unpredictable `crypto/rand` short codes with collision retry.
> 10 tests (including 18 table-driven subtests) run under the race detector on every push via GitHub Actions.

![demo](docs/demo.gif)

## 功能簡介

- **建立短網址**：`POST /shorten` 送出網址，回傳 6 碼短碼與完整的短網址（`201 Created`）。
- **轉址**：`GET /{code}` 以 `302 Found` 轉址到原本的網址；短碼不存在時回傳 404。
- **輸入驗證**：只接受有 host 的 http/https 網址。
- **統一錯誤格式**：錯誤一律以 JSON 格式回傳；500 的內部細節只寫入日誌，不回傳給客戶端。

## 快速開始

### 需要先安裝

**Go 1.22 以上**：[go.dev/dl](https://go.dev/dl/)。

### 步驟

```bash
# 1. 啟動服務（預設 port 8080）
go run ./cmd/server

# 2. 在另一個終端機建立短網址
curl -X POST localhost:8080/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://github.com/kimi0217/go-url-shortener"}'
```

把回傳的 `short_url` 貼到瀏覽器，就會跳轉到原本的網址。

### 環境變數

| 環境變數 | 預設值 | 說明 |
|---|---|---|
| `PORT` | `8080` | 監聽的埠號 |
| `BASE_URL` | `http://localhost:$PORT` | 回應中 `short_url` 的前綴。部署後對外網址和實際監聽的位址不同時使用，例如設成 `https://sho.rt`，回傳的短網址就會是 `https://sho.rt/abc123` |

## API

### `POST /shorten`：建立短網址

```
$ curl -i -X POST localhost:8080/shorten \
    -H 'Content-Type: application/json' \
    -d '{"url":"https://go.dev/doc/"}'

HTTP/1.1 201 Created
Content-Type: application/json

{"code":"CmeBbR","short_url":"http://localhost:8080/CmeBbR"}
```

### `GET /{code}`：轉址

```
$ curl -i localhost:8080/CmeBbR

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
| 500 | 內部錯誤（細節寫入日誌） |

## 專案結構

```
cmd/server/          進入點：讀取設定、組裝依賴、啟動 http.Server
internal/handler/    HTTP handlers 與 table-driven 測試
internal/store/      Store interface 與記憶體實作
internal/shortener/  短碼產生
docs/                demo GIF 與錄製腳本（VHS）
.github/workflows/   CI：go vet、go test -race
```

## 測試

```bash
go test ./...          # 執行所有測試
go test -race ./...    # 加上 race detector
go vet ./...           # 靜態檢查
```

## 設計說明

- **`Store` interface**：handler 只依賴 interface，不綁定特定的儲存方式，測試時可以換成假的實作，日後也能直接換成資料庫。
- **併發安全**：記憶體實作使用 `sync.RWMutex`。`Save` 在同一把寫鎖內完成「檢查是否存在」和「寫入」，避免兩個請求同時搶到同一個短碼（check-then-act race）。
- **短碼產生**：使用 `crypto/rand` 產生 6 碼英數字（62⁶ ≈ 570 億種組合），短碼無法被預測或列舉。
- **碰撞處理**：`Store.Save` 遇到重複的短碼時回傳 `ErrCodeExists`，handler 重新產生短碼後重試，最多 5 次。
- **輸入驗證**：只接受有 host 的 http/https 網址，擋掉 `javascript:` 等 scheme，避免服務被拿來轉址到惡意連結。
- **302 而非 301**：瀏覽器會永久快取 301，之後的請求就不會再經過這個服務。
- **Server 逾時**：使用自訂的 `http.Server`，設定 `ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout`、`IdleTimeout`，避免慢速客戶端（例如 Slowloris）長時間佔住連線。
- **日誌**：使用 `log/slog` 輸出 JSON 格式。

## 已知限制

- 資料存在記憶體，重啟後全部遺失。
- 沒有對應到任何路由的請求（例如 `GET /`、`DELETE /abc`）由 `ServeMux` 直接回傳純文字的 404/405，不是 JSON 格式。
- 同一個網址送出多次，會產生多個不同的短碼。