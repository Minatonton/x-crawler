.PHONY: build run clean test install

# ビルド
build:
	go build -o x-crawler

# 実行
run: build
	./x-crawler

# クリーンアップ
clean:
	rm -f x-crawler
	rm -f seen_tweets.json

# テスト
test:
	go test -v ./...

# 依存関係の更新
deps:
	go mod tidy
	go mod download

# Linuxバイナリのビルド（GCE用）
build-linux:
	GOOS=linux GOARCH=amd64 go build -o x-crawler-linux

# インストール（systemdサービス化）
install: build-linux
	sudo cp x-crawler-linux /usr/local/bin/x-crawler
	sudo cp x-crawler.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable x-crawler
	sudo systemctl start x-crawler

# ログ確認
logs:
	sudo journalctl -u x-crawler -f

# ステータス確認
status:
	sudo systemctl status x-crawler
