all: arm ios android 

local:
	CGO_ENABLED=0 go build -o build/perception cmd/perception/main.go
	@echo "local for dev compilation done: run as 'build/perception'"


darwin:
	xgo -targets=darwin/amd64 -v -out build/perception ./cmd/perception/
	@echo "Darwin amd64 cross compilation done:"

windows:
	xgo -targets=windows/amd64 -v -out build/perception ./cmd/perception/
	@echo "Windows amd64 cross compilation done:"

linux:
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -ldflags "-w -s" -o build/perception-linux-amd64 cmd/perception/main.go
	tar czvf build/perception-linux-amd64.tar.gz build/perception-linux-amd64
	@echo "Linux linux-x86 cross compilation done:"
arm-debug:
	CGO_ENABLED=0 GOARCH=arm GOOS=linux go build -o build/perception-linux-arm-debug cmd/perception/main.go
	@echo "Linux ARM cross compilation done:"

arm:
	CGO_ENABLED=0 GOARCH=arm GOOS=linux go build -ldflags "-w -s" -o build/perception-linux-arm cmd/perception/main.go
	tar czvf build/perception-linux-arm.tar.gz build/perception-linux-arm
	@echo "Linux ARM cross compilation done:"

ios:
	gomobile bind -ldflags "-w -s" -target=ios -o=build/mobile.framework github.com/SmartMeshFoundation/Perception/cmd/mobile
	tar czvf build/mobile.framework.tar.gz build/mobile.framework
	@echo "ios cross compilation done:"

android:
	gomobile bind -ldflags "-w -s" -target=android -o=build/mobile.aar github.com/SmartMeshFoundation/Perception/cmd/mobile
	@echo "android cross compilation done:"

clean:
	rm -rf build/*
