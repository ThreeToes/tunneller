BINARY_NAME=tunneller
OUTPUT_PATH=build
STACK_NAME=ecs-fargate-$(MODULE_NAME)
ifeq ($(OS),Windows_NT)
    EXE_SUFFIX=.exe
    EXPORT=set
else
	EXE_SUFFIX=
	EXPORT=export
endif

.PHONY: build test vendor

build: bin
	zip $(OUTPUT_PATH)/tunneller-windows $(OUTPUT_PATH)/windows/$(BINARY_NAME).exe
	zip $(OUTPUT_PATH)/tunneller-linux $(OUTPUT_PATH)/linux/$(BINARY_NAME)
	zip $(OUTPUT_PATH)/tunneller-macos $(OUTPUT_PATH)/macos/$(BINARY_NAME)

windows:
	$(EXPORT) GOOS=windows
	go build -mod=vendor -o $(OUTPUT_PATH)/windows/$(BINARY_NAME).exe ./cmd/...

linux:
	$(EXPORT) GOOS=linux
	go build -mod=vendor -o $(OUTPUT_PATH)/linux/$(BINARY_NAME) ./cmd/...

macos:
	$(EXPORT) GOOS=darwin
	go build -mod=vendor -o $(OUTPUT_PATH)/macos/$(BINARY_NAME) ./cmd/...

bin: test windows linux macos

test: vendor
	go test -mod=vendor -cover-profile=coverage.out ./internal/...

vendor:
	go mod tidy
	go mod vendor