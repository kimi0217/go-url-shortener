# CLAUDE.md

## 專案概述

`go-url-shortener` 是一個用 Go 撰寫的簡易短網址 REST API，作為求職作品集與 Go 學習專案。
開發時間預算約一天，請維持「小而完整」，不要擴充範圍。

## 使用者背景（重要）

- 我是 Go 初學者，這個專案的首要目的是**學習**，其次才是完成。
- 每寫完一個檔案或一段邏輯，請用繁體中文簡短說明：做了什麼、為什麼這樣寫、用到哪些 Go 觀念。
- 程式碼註解用英文，但要解釋「為什麼」，不只是「做什麼」。
- 遇到 Go 特有的寫法（interface、error handling、goroutine、defer、struct embedding 等），請主動點出來。
- 一次只做一個小步驟，完成後停下來讓我確認，不要一口氣寫完整個專案。

## 功能範圍

### 必做
- `POST /shorten`：接收 JSON `{"url": "https://..."}`，回傳 `{"code": "abc123", "short_url": "http://localhost:8080/abc123"}`
- `GET /{code}`：302 重新導向到原網址；找不到時回傳 404
- 輸入驗證：URL 必須是合法的 http/https 網址，否則回傳 400
- 記憶體儲存，透過 `Store` interface 抽象
- handler 的 table-driven 測試
- README

### 有時間再做
- GitHub Actions（`go vet`、`go test`）
- Dockerfile（multi-stage build）

### 不要做
- 資料庫、點擊統計、rate limiting、使用者驗證、前端頁面
- 不要引入第三方框架（Gin、Echo、Chi 等）

## 技術規範

- Go 版本：1.22 以上（使用標準庫 `net/http` 的新路由語法，例如 `mux.HandleFunc("GET /{code}", ...)`）
- **只使用標準庫**，不加任何外部依賴
- 日誌使用 `log/slog`
- 併發安全：記憶體儲存使用 `sync.RWMutex`
- 短碼產生：使用 `crypto/rand`，長度 6，字元集為英數字；碰撞時重試
- 錯誤回應統一為 JSON：`{"error": "message"}`
- 埠號從環境變數 `PORT` 讀取，預設 `8080`

## 目錄結構

```
go-url-shortener/
├── cmd/server/main.go        # 進入點：組裝依賴、啟動 server
├── internal/
│   ├── handler/              # HTTP handlers 與測試
│   ├── store/                # Store interface 與 memory 實作
│   └── shortener/            # 短碼產生邏輯
├── go.mod
├── README.md
└── CLAUDE.md
```

## 常用指令

```bash
go run ./cmd/server           # 啟動服務
go test ./...                 # 執行所有測試
go test -race ./...           # 含 race detector
go vet ./...                  # 靜態檢查
gofmt -l .                    # 檢查格式
```

## 工作流程

- 每完成一個有意義的步驟就 commit，訊息使用 Conventional Commits 格式（`feat:`、`test:`、`docs:`、`chore:`）。
- commit 前先執行 `gofmt`、`go vet ./...`、`go test ./...`，全部通過才 commit。
- 不要自行 push，push 由我手動執行。

## 建議開發順序

1. `go.mod` 與目錄結構
2. `internal/store`：`Store` interface + memory 實作 + 測試
3. `internal/shortener`：短碼產生 + 測試
4. `internal/handler`：兩個 handler + table-driven 測試
5. `cmd/server/main.go`：組裝並啟動
6. README（含 curl 範例與設計說明）
7. （選做）GitHub Actions、Dockerfile
