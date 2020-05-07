BINARY_NAME=tunneller
OUTPUT_PATH=build
STACK_NAME=ecs-fargate-$(MODULE_NAME)
ifeq ($(OS),Windows_NT)
    EXE_SUFFIX=.exe
else
	EXE_SUFFIX=
endif

build: test
	go build -mod=vendor -o $(OUTPUT_PATH)/$(BINARY_NAME)$(EXE_SUFFIX) ./cmd/...

test: vendor
	go test -mod=vendor cover.out ./internal/...

vendor:
	go mod tidy
	go mod vendor