BINARY_NAME := finops-guard
BUILD_DIR   := bin
CMD_DIR     := ./cmd/finops-guard
PRICING_FILE := pricing.json
SAMPLE_FILE := test/testdata/ai_slop_sample.py

.PHONY: build run demo test cover vet fmt clean

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

run: build
	./$(BUILD_DIR)/$(BINARY_NAME) --pricing=$(PRICING_FILE) --scan=$(SAMPLE_FILE) --no-tui

demo: build
	./$(BUILD_DIR)/$(BINARY_NAME) --pricing=$(PRICING_FILE) --scan=$(SAMPLE_FILE)

test:
	go test -v ./...

cover:
	go test -cover ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -rf $(BUILD_DIR)
