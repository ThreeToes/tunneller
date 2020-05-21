BINARY_NAME=tunneller
OUTPUT_PATH=build
STACK_NAME=ecs-fargate-$(MODULE_NAME)
ifeq ($(OS),Windows_NT)
    EXE_SUFFIX=.exe
else
	EXE_SUFFIX=
endif

.PHONY: build test vendor

build: bin
	zip $(OUTPUT_PATH)/tunneller $(OUTPUT_PATH)/$(BINARY_NAME)$(EXE_SUFFIX)

bin: test
	go build -mod=vendor -o $(OUTPUT_PATH)/$(BINARY_NAME)$(EXE_SUFFIX) ./cmd/...

test: vendor
	go test -mod=vendor -cover-profile=coverage.out ./internal/...

vendor:
	go mod tidy
	go mod vendor