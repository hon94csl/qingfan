WAILS = go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0

.PHONY: setup dev build test doctor server admin-dev admin-build trial
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
	npm --prefix frontend test
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

trial:
	rm -rf build/trial
	mkdir -p build/trial/bin build/trial/fonts
	cp "$$(command -v ffmpeg)" build/trial/bin/ffmpeg
	cp "$$(command -v ffprobe)" build/trial/bin/ffprobe
	chmod +x build/trial/bin/ffmpeg build/trial/bin/ffprobe
	cp tests/fixtures/fonts/Lato-Regular.ttf tests/fixtures/fonts/OFL.txt build/trial/fonts/
	build/trial/bin/ffmpeg -version > build/trial/ffmpeg-version.txt
	build/trial/bin/ffprobe -version > build/trial/ffprobe-version.txt
	cp docs/trial/README.md build/trial/README.md
