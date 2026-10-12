WAILS = go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0

.PHONY: setup dev build test doctor server admin-dev admin-build
setup:
	go mod download
	npm --prefix frontend ci
	npm --prefix admin ci

dev:
	$(WAILS) dev

build:
	$(WAILS) build

test:
	go test ./...
	npm --prefix admin test
	npm --prefix frontend run build
	npm --prefix admin run build

doctor:
	go run ./cmd/qingfan-render doctor

server:
	go run ./cmd/qingfan-server

admin-dev:
	npm --prefix admin run dev

admin-build:
	npm --prefix admin run build
